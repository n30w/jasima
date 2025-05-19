package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/network"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/pkg/errors"
)

// iterate begins the processing of a Layer. The function completes after the
// total number of back and forth rounds are complete. Layer control and message
// routing are decoupled.
func (s *ConlangServer) iterate(
	ctx context.Context,
	initialGeneration memory.Generation,
	initialLayer chat.Layer,
	exchanges int,
) (memory.Generation, error) {
	// The maps and slices need to be copied, since in Go, maps and slices are
	// pass by reference always.

	newGeneration := memory.Generation{
		Transcript:     newTranscriptGeneration(),
		Logography:     initialGeneration.Logography.Copy(),
		Specifications: initialGeneration.Specifications.Copy(),
		Dictionary:     initialGeneration.Dictionary.Copy(),
	}

	if initialLayer == chat.SystemLayer {
		s.logger.Info("initial layer is system")
		return newGeneration, nil
	}

	s.logger.Infof("%d: Recursing on %s", initialLayer, initialLayer)

	prevGeneration, err := s.iterate(
		ctx,
		initialGeneration,
		initialLayer-1,
		exchanges,
	)
	if err != nil {
		return initialGeneration, errors.Wrapf(
			err,
			"failed iteration at %s",
			initialLayer,
		)
	}

	sysClient, err := s.gs.GetClientByName("SYSTEM_AGENT_A")
	if err != nil {
		return newGeneration, errors.Wrap(
			err,
			"failed to retrieve client by name",
		)
	}

	var (
		timer   = utils.Timer(time.Now())
		clients = s.gs.GetClientsByLayer(initialLayer)

		// kickoff is the command that is sent to kick off the layer's
		// iteration exchanges. It consists of the initializer client, which
		// is the first client of the layer, and content, which is the text
		// in the message that the initializer client will send to other
		// clients on the layer.
		kickoff = s.cmd(
			agent.SendInitialMessage,
			fmt.Sprintf(
				"Hello, "+
					"let's begin developing Toki Pona %s. You go first.",
				initialLayer,
			),
		)(s.gs.GetClientsByLayer(initialLayer)[0])

		// initialInstructions are the initial instructions each agent on the
		// layer is given for its system prompt.
		initialInstructions = fmt.Sprintf(
			"You and your interlocutors are responsible for developing %s. Reason and discuss using the current specification."+
				"\nHere is the current specification for %s.\n",
			initialLayer,
			initialLayer,
		)

		// addSysAgentInstructions is a command to append the current
		// specification to the system agent(s).
		addSysAgentInstructions = s.cmd(
			agent.AppendInstructions,
			fmt.Sprintf(
				"The current specification is for: %s. "+
					"Here is the current specification:\n%s",
				initialLayer,
				initialGeneration.Specifications[initialLayer].String(),
			),
		)(sysClient)

		layerSpecificInstructions = map[chat.Layer]string{
			chat.SystemLayer:    "",
			chat.PhoneticsLayer: "",
			chat.GrammarLayer:   "",
			chat.DictionaryLayer: "Do not discuss the structure of the" +
				" dictionary. Rather, " +
				"discuss the words and enhancements that may need to be made to" +
				" them.",
			chat.LogographyLayer: "",
		}

		sb strings.Builder
	)

	sendCommands := network.SendCommandBuilder(ctx, s.gs.Channel.ToClients)

	newGeneration.Transcript = prevGeneration.Transcript.Copy()
	newGeneration.Specifications = prevGeneration.Specifications.Copy()
	newGeneration.Dictionary = prevGeneration.Dictionary.Copy()

	sb.WriteString(initialInstructions)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newGeneration.Specifications[i].String())
		sb.WriteString("\n")
	}

	// Add all words in the language.

	var wordsAndGrammar string
	{
		sb.WriteString(
			"Here is the complete dictionary of all words in the" +
				" language:\n",
		)
		sb.WriteString(newGeneration.Dictionary.String())
		sb.WriteString(layerSpecificInstructions[initialLayer])
		sb.WriteString("\n")

		// Add all grammar in the language.

		sb.WriteString("Here is the complete grammar of the language:\n")
		sb.WriteString(string(newGeneration.Specifications[chat.GrammarLayer]))

		wordsAndGrammar = sb.String()
	}

	sendCommands(
		clients,
		s.cmd(agent.AppendInstructions, wordsAndGrammar),
		s.cmd(agent.Unlatch),
	)

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	err = s.swc(ctx, kickoff)
	if err != nil {
		return newGeneration, err
	}

	select {
	case <-ctx.Done():
		return newGeneration, ctx.Err()
	default:
		for i := range exchanges {
			select {
			case <-ctx.Done():
				return newGeneration, nil
			case m := <-s.procedureChan:
				newGeneration.Transcript[initialLayer] = append(
					newGeneration.Transcript[initialLayer],
					m,
				)

				// Retrieve the extracted words.

				usedWords, err := s.extractUsedWords(ctx, newGeneration.Dictionary, m.Text.String())
				if err != nil {
					return newGeneration, errors.Wrap(err, "failed finding used words")
				}

				// Broadcast the sent message.

				s.ws.Broadcasters.Messages.Broadcast(m)

				// Broadcast the extracted words from the sent message.

				s.ws.Broadcasters.MessageWordDictExtraction.Broadcast(usedWords)

				s.logger.Infof("Exchange Total: %d/%d", i+1, exchanges)
			}
		}
	}

	s.resetAgents(ctx, clients)

	sb.Reset()

	// The system agent will ONLY summarize the chat log, and not read the
	// other Specifications for other layers (for now at least).

	err = s.swc(ctx, addSysAgentInstructions)
	if err != nil {
		return newGeneration, err
	}

	err = s.swc(ctx, s.cmd(agent.Unlatch)(sysClient))
	if err != nil {
		return newGeneration, err
	}

	msg := s.messageToSystemAgent(
		sysClient.Name,
		transcriptToString(newGeneration.Transcript[initialLayer]),
	)

	err = s.swc(ctx, msg)
	if err != nil {
		return newGeneration, err
	}

	select {
	case <-ctx.Done():
		return newGeneration, nil
	case specPrime := <-s.gs.Channel.ToServer:
		sb.Reset()

		s.resetAgent(ctx, sysClient)

		s.logger.Infof("%s took %s to complete", initialLayer, timer())

		newGeneration.Specifications[initialLayer] = specPrime.Text
		s.ws.Broadcasters.Specification.Broadcast(newGeneration.Specifications)

		// End of side effects.

		if initialLayer == chat.DictionaryLayer {
			s.iterateUpdateDictionary(ctx, newGeneration)
		}

		return newGeneration, nil
	}
}

func (s *ConlangServer) iterateUpdateDictionary(
	ctx context.Context,
	newGeneration memory.Generation,
) {
	s.logger.Info("Initiating dictionary updates...")

	dictSysAgent, err := s.gs.GetClientByName("SYSTEM_AGENT_B")
	if err != nil {
		s.errs <- err
		return
	}

	clients := []*network.ChatClient{dictSysAgent}

	genDict, err := json.Marshal(newGeneration.Dictionary)
	if err != nil {
		s.errs <- errors.Wrap(err, "failed to Marshal dictionary")
		return
	}

	err = s.swc(ctx, s.cmd(agent.Latch)(dictSysAgent))
	if err != nil {
		s.errs <- err
		return
	}

	currentDict := s.cmd(
		agent.AppendInstructions, fmt.Sprintf(
			"This is the current dictionary\n%s",
			string(genDict),
		),
	)(dictSysAgent)

	err = s.swc(ctx, currentDict)
	if err != nil {
		s.errs <- err
		return
	}

	// Send results to dictionary LLM.

	err = s.swc(ctx, s.cmd(agent.Unlatch)(dictSysAgent))
	if err != nil {
		s.errs <- err
		return
	}

	dictUpdateRequest := s.cmd(
		agent.RequestJsonDictionaryUpdate,
		transcriptToString(newGeneration.Transcript[chat.DictionaryLayer]),
	)(dictSysAgent)

	err = s.swc(ctx, dictUpdateRequest)
	if err != nil {
		s.errs <- err
		return
	}

	var updates memory.ResponseDictionaryEntries

	select {
	case <-ctx.Done():
		return
	case dictUpdates := <-s.gs.Channel.ToServer:
		err = json.Unmarshal([]byte(dictUpdates.Text), &updates)
		if err != nil {
			s.errs <- errors.Wrap(
				err,
				"failed to unmarshal dictionary updates",
			)
			return
		}
	}

	select {
	case <-ctx.Done():
		return
	case s.dictUpdatesChan <- updates:
		s.logger.Info("Updates sent to dictionary channel")
	}

	s.resetAgents(ctx, clients)
}

func (s *ConlangServer) iterateLogogram(
	ctx context.Context,
	newGeneration memory.Generation,
	word string,
) (
	string,
	error,
) {
	genericInstructions := "\nHere is the current dictionary:\n" + newGeneration.Specifications[chat.DictionaryLayer].String() + "\nHere is the current logography specification:\n" + newGeneration.Specifications[chat.LogographyLayer].String()

	generator, err := s.gs.GetClientByName("SYSTEM_AGENT_D")
	if err != nil {
		return "", errors.Wrap(err, "failed to find generator agent")
	}

	adversary, err := s.gs.GetClientByName("SYSTEM_AGENT_E")
	if err != nil {
		return "", errors.Wrap(err, "failed to find adversary agent")
	}

	var (
		i          = 0
		clients    = []*network.ChatClient{generator, adversary}
		currentSvg = ""

		generatorInstructions = s.cmd(
			agent.SetInstructions,
			agent.LogogramGeneratorInstructions+genericInstructions,
		)(generator)
		adversaryInstructions = s.cmd(
			agent.SetInstructions,
			agent.LogogramAdversaryInstructions+genericInstructions,
		)(adversary)

		generatorOk = false
		adversaryOk = false
	)

	sendCommands := network.SendCommandBuilder(ctx, s.gs.Channel.ToClients)

	// Fresh slate.

	sendCommands(clients, s.cmd(agent.Latch), s.cmd(agent.ClearMemory))

	err = s.swc(ctx, generatorInstructions)
	if err != nil {
		return "", errors.Wrap(err, "failed to send generator instructions")
	}

	err = s.swc(ctx, adversaryInstructions)
	if err != nil {
		return "", errors.Wrap(err, "failed to send adversary instructions")
	}

	sendCommands(clients, s.cmd(agent.Unlatch))

	// Send the initial message.

	initMsg := memory.ResponseLogogramIteration{
		Name: word,
		Svg:  s.dictionary[word].Logogram,
		ResponseText: memory.ResponseText{
			Response: "This is the initial svg. We will be developing logogram for the word: " + word,
		},
	}

	initMsgJson, err := json.Marshal(initMsg)
	if err != nil {
		return "", err
	}

	kickoff := s.cmd(agent.SendInitialMessage, string(initMsgJson))(generator)

	err = s.swc(ctx, kickoff)
	if err != nil {
		return "", errors.Wrap(err, "failed to send kickoff message")
	}

	logoIter := memory.LogogramIteration{
		Generator: initMsg,
		Adversary: memory.ResponseLogogramCritique{},
	}

	err = s.ws.InitialData.RecentLogogram.Enqueue(logoIter)
	if err != nil {
		return "", err
	}

	s.ws.Broadcasters.LogogramDisplay.Broadcast(logoIter)

	// In case the agents go out of control, cap `i` at `DefaultMaxExchanges`

	for (!adversaryOk || !generatorOk) && i <= DefaultMaxExchanges {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case m := <-s.gs.Channel.ToServer:
			var msg *chat.Message

			logoIter = memory.LogogramIteration{
				Generator: logoIter.Generator,
				Adversary: logoIter.Adversary,
			}

			// Switch the message to the recipient based on the sender. If the
			// sender is the generator, rewrite the response into one for
			// the adversary. If the sender is the adversary, rewrite the message
			// for the generator.

			switch m.Sender {
			case generator.Name:

				// Make a message for the adversary.

				var res memory.ResponseLogogramIteration

				err := json.Unmarshal([]byte(m.Text), &res)
				if err != nil {
					return "", errors.Wrap(err, "failed to unmarshal agent logogram iteration")
				}

				logoIter.Generator = res

				currentSvg = res.Svg

				generatorOk = res.Stop

				msg = s.cmd(
					agent.RequestLogogramCritique,
					res.Name+"\n"+res.Svg+"\n\n"+res.Response,
				)(adversary)

				// Validate SVG, send to sys agent for correction.

			case adversary.Name:

				// Make a message for the generator.

				var res memory.ResponseLogogramCritique

				err := json.Unmarshal([]byte(m.Text), &res)
				if err != nil {
					return "", errors.Wrap(err, "failed to unmarshal generator logogram critique")
				}

				logoIter.Adversary = res

				adversaryOk = res.Stop

				msg = s.cmd(agent.RequestLogogramIteration, res.Response)(generator)
			}

			usedWords, err := s.extractUsedWords(ctx, newGeneration.Dictionary, m.Text.String())
			if err != nil {
				return "", errors.Wrap(err, "failed finding used words")
			}

			// Broadcast the sent message.

			s.ws.Broadcasters.Messages.Broadcast(m)

			// Broadcast the extracted words from the sent message.

			s.ws.Broadcasters.MessageWordDictExtraction.Broadcast(usedWords)

			err = s.ws.InitialData.RecentLogogram.Enqueue(logoIter)
			if err != nil {
				return "", err
			}

			s.ws.Broadcasters.LogogramDisplay.Broadcast(logoIter)

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Second * 10):
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case s.gs.Channel.ToClients <- msg:
				// `i` is incremented here because an exchange is only when a message
				// traverses the boundary of one agent to another.

				i++

				s.logger.Debugf("Exchanges: %d", i)
			}
		}
	}

	// Put everything back.

	s.resetAgents(ctx, clients)

	return currentSvg, nil
}

func (s *ConlangServer) WaitForClients(total int) Job {
	return func(ctx context.Context) error {
		var err error

		joinCtx, joinCtxCancel := context.WithCancel(ctx)

		defer joinCtxCancel()

		go func() {
			defer joinCtxCancel()
			s.logger.Info("Waiting for clients to join...")
			for {
				select {
				case <-joinCtx.Done():
					err = joinCtx.Err()
					return
				case <-time.After(time.Second):
					if s.gs.TotalClients() >= total {
						s.logger.Info("All clients joined!")
						return
					}
				}
			}
		}()

		<-joinCtx.Done()

		if err != nil {
			return err
		}

		return nil
	}
}

func (s *ConlangServer) iterateSpecs(i int, g *memory.Generation) Job {
	return func(ctx context.Context) error {
		var err error

		genSlice, err := s.generations.ToSlice()
		if err != nil {
			return errors.Wrap(err, "failed to iterate specs")
		}

		// Starts on Layer 4, recurses to 1.

		*g, err = s.iterate(
			ctx,
			genSlice[i],
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			return errors.Wrapf(err, "failed to iterate on generation %d", i)
		}

		s.logger.Infof("Generation %d specs completed ", i+1)

		return nil
	}
}

func (s *ConlangServer) iterateLogograms(i int, g *memory.Generation) Job {
	return func(ctx context.Context) error {
		// Get the transcript of the logography layer.
		t := g.Transcript[chat.LogographyLayer]

		// Extract logogram names from logography transcript.
		res, err := s.findUsedWords(ctx, g.Dictionary.Copy(), t.String())
		if err != nil {
			return err
		}

		// For each used word, iterate it.

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:

			// It can be made so that agents do not clear their memory on each
			// iteration of a word to keep a long-running context window, but
			// I currently do not have the money or compute for that, and I
			// don't know if I ever will.

			for _, word := range res.Words {
				svg, err := s.iterateLogogram(ctx, *g, word)
				if err != nil {
					return errors.Wrapf(err, "failed to iterate logograms on iteration %d", i)
				}

				// This will automatically update the svg, so in the next iteration
				// the agents will have access to their previous developed
				// logograms.

				g.Logography[word] = svg
			}

			s.dictionary = g.Dictionary.Copy()

			return nil
		}
	}
}

func (s *ConlangServer) updateGenerations(i int, g *memory.Generation) Job {
	return func(ctx context.Context) error {
		err := s.generations.Enqueue(*g)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to enqueue new generation %d", i,
			)
		}

		err = s.ws.InitialData.RecentGenerations.Enqueue(*g)
		if err != nil {
			return errors.Wrapf(
				err,
				"failed to enqueue new generation to initial data %d", i,
			)
		}

		s.ws.Broadcasters.Generation.Broadcast(*g)

		return nil
	}
}

func (s *ConlangServer) iterateDictionary(i int, g *memory.Generation) Job {
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case updates := <-s.dictUpdatesChan:

			s.logger.Info("Received dictionary update")

			// Update the generation's dictionary based on updates.

			currentDict := g.Dictionary.Copy()

			for _, update := range updates.Entries {

				if update.Remove {
					_, ok := currentDict[update.Word]
					if !ok {
						s.logger.Warnf(
							"%s not in dictionary, skipping",
							update.Word,
						)
						continue
					}

					delete(currentDict, update.Word)
					continue
				}

				entry := currentDict[update.Word]

				entry.Word = update.Word
				entry.Definition = update.Definition

				currentDict[update.Word] = entry
			}

			g.Dictionary = currentDict.Copy()
		}

		return nil
	}
}

func (s *ConlangServer) wait(t time.Duration) Job {
	return func(ctx context.Context) error {
		s.logger.Infof("Waiting for %s...", t)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t):
		}

		return nil
	}
}

func (s *ConlangServer) exportData(t func() time.Duration) Job {
	return func(ctx context.Context) error {
		s.gs.Listening = false

		s.logger.Info("EVOLUTION COMPLETE")
		s.logger.Infof("Evolution took %s", t())

		// Marshal to JSON

		allMsgs, err := s.memory.All()
		if err != nil {
			return err
		}

		fileName := fmt.Sprintf(
			"./outputs/chats/chat_%s.json",
			time.Now().Format("20060102150405"),
		)

		err = saveToJson(allMsgs, fileName)
		if err != nil {
			return errors.Wrap(err, "evolution failed to save JSON")
		}

		s.logger.Infof("Saved chat to %s", fileName)

		fileName = fmt.Sprintf(
			"./outputs/generations/generations_%s.json",
			time.Now().Format("20060102150405"),
		)

		g, err := s.generations.ToSlice()
		if err != nil {
			return errors.Wrap(err, "evolution failed to save JSON")
		}

		err = saveToJson(g, fileName)
		if err != nil {
			return errors.Wrap(err, "evolution failed to save JSON")
		}

		s.logger.Infof("Saved generations to %s", fileName)

		return nil
	}
}

func (s *ConlangServer) TestIterateLogogram(ctx context.Context) error {
	g, err := s.generations.ToSlice()
	if err != nil {
		return err
	}

	svg, err := s.iterateLogogram(ctx, g[0], "suli")
	if err != nil {
		return err
	}

	s.logger.Printf("%s\n", svg)

	return nil
}

func (s *ConlangServer) SelfDestruct(_ context.Context) error {
	return errors.New("SERVER KILLED FROM SELF-DESTRUCTION")
}

// outputTestData continuously outputs messages to the test API. This is
// useful for frontend testing without having to run agent queries
// over and over again.
func (s *ConlangServer) outputTestData(
	ctx context.Context,
	messages []memory.Message,
	generations []memory.Generation,
) {
	var (
		i1 = 0
		i2 = 0
		l1 = len(messages)
		l2 = len(generations)
		t1 = time.NewTicker(time.Second)
		t2 = time.NewTicker(3 * time.Second)
	)

	dataCtx, dataCancel := context.WithCancel(ctx)

	defer dataCancel()

	s.logger.Info("Emitting test output data...")

	go func() {
		ctx, cancel := context.WithCancel(dataCtx)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		default:
			for {
				<-t2.C
				s.ws.Broadcasters.TestGenerationsFeed.Broadcast(generations[i2%l2])
				i2++
			}
		}
	}()

	go func() {
		ctx, cancel := context.WithCancel(dataCtx)
		defer cancel()
		select {
		case <-ctx.Done():
			return
		default:
			for {
				<-t1.C
				s.ws.Broadcasters.TestMessageFeed.Broadcast(messages[i1%l1])
				i1++
			}
		}
	}()
}
