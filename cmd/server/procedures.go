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

	newGeneration.Transcript = prevGeneration.Transcript.Copy()
	newGeneration.Specifications = prevGeneration.Specifications.Copy()
	newGeneration.Dictionary = prevGeneration.Dictionary.Copy()

	sb.WriteString(initialInstructions)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newGeneration.Specifications[i].String())
		sb.WriteString("\n")
	}

	// Add all words in the language.

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

	s.sendCommands(
		clients,
		s.cmd(agent.AppendInstructions, sb.String()),
		s.cmd(agent.Unlatch),
	)

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	s.gs.Channel.ToClients <- kickoff

	for i := range exchanges {

		m := <-s.procedureChan

		newGeneration.Transcript[initialLayer] = append(
			newGeneration.Transcript[initialLayer],
			m,
		)

		// Retrieve the extracted words.

		usedWords, err := s.extractUsedWords(newGeneration.Dictionary, m.Text.String())
		if err != nil {
			return newGeneration, errors.Wrap(err, "failed finding used words")
		}

		// Broadcast the sent message.

		s.ws.Broadcasters.Messages.Broadcast(m)

		// Broadcast the extracted words from the sent message.

		s.ws.Broadcasters.MessageWordDictExtraction.Broadcast(usedWords)

		s.logger.Infof("Exchange Total: %d/%d", i+1, exchanges)
	}

	s.resetAgents(clients)

	sb.Reset()

	// The system agent will ONLY summarize the chat log, and not read the
	// other Specifications for other layers (for now at least).

	s.gs.Channel.ToClients <- addSysAgentInstructions
	s.gs.Channel.ToClients <- s.cmd(agent.Unlatch)(sysClient)

	msg := s.messageToSystemAgent(
		sysClient.Name,
		transcriptToString(newGeneration.Transcript[initialLayer]),
	)

	s.gs.Channel.ToClients <- msg

	s.logger.Infof("Waiting for system agent response...")

	specPrime := <-s.gs.Channel.ToServer

	sb.Reset()

	s.resetAgents(clients)

	s.logger.Infof(
		"%s took %s to complete",
		initialLayer,
		timer().Truncate(time.Millisecond),
	)

	// End of side effects.

	newGeneration.Specifications[initialLayer] = specPrime.Text

	// JK some more.

	s.ws.Broadcasters.Specification.Broadcast(newGeneration.Specifications)

	if initialLayer == chat.DictionaryLayer {
		// Update the dictionary.

		s.iterateUpdateDictionary(newGeneration)
	}

	return newGeneration, nil
}

func (s *ConlangServer) iterateUpdateDictionary(
	newGeneration memory.Generation,
) {
	s.logger.Info("Initiating dictionary updates...")

	dictSysAgent, err := s.gs.GetClientByName("SYSTEM_AGENT_B")
	if err != nil {
		s.errs <- err
		return
	}

	clients := []*network.GRPCClient{dictSysAgent}

	genDict, err := json.Marshal(newGeneration.Dictionary)
	if err != nil {
		s.errs <- errors.Wrap(err, "failed to Marshal dictionary")
		return
	}

	s.gs.Channel.ToClients <- s.cmd(agent.Latch)(dictSysAgent)
	s.gs.Channel.ToClients <- s.cmd(
		agent.AppendInstructions,
		fmt.Sprintf(
			"This is the current dictionary\n%s",
			string(genDict),
		),
	)(dictSysAgent)

	// Send results to dictionary LLM.

	s.gs.Channel.ToClients <- s.cmd(agent.Unlatch)(dictSysAgent)
	s.gs.Channel.ToClients <- s.cmd(
		agent.RequestJsonDictionaryUpdate,
		transcriptToString(newGeneration.Transcript[chat.DictionaryLayer]),
	)(dictSysAgent)

	var updates chat.DictionaryEntriesResponse

	dictUpdates := <-s.gs.Channel.ToServer

	err = json.Unmarshal([]byte(dictUpdates.Text), &updates)
	if err != nil {
		s.errs <- errors.Wrap(
			err,
			"failed to unmarshal dictionary updates",
		)
		return
	}

	s.dictUpdatesChan <- updates

	s.logger.Info("Updates sent to dictionary channel")

	s.resetAgents(clients)
}

func (s *ConlangServer) iterateLogogram(newGeneration memory.Generation, word string) (
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
		done       = false
		i          = 0
		clients    = []*network.GRPCClient{generator, adversary}
		currentSvg = ""

		generatorInstructions = s.cmd(
			agent.SetInstructions,
			agent.LogogramGeneratorInstructions+genericInstructions,
		)(generator)
		adversaryInstructions = s.cmd(
			agent.SetInstructions,
			agent.LogogramAdversaryInstructions+genericInstructions,
		)(adversary)
	)

	// Fresh slate.

	s.sendCommands(clients, s.cmd(agent.Latch), s.cmd(agent.ClearMemory))

	s.gs.Channel.ToClients <- generatorInstructions
	s.gs.Channel.ToClients <- adversaryInstructions

	s.sendCommands(clients, s.cmd(agent.Unlatch))

	// Send the initial message.

	initMsg := chat.AgentLogogramIterationResponse{
		Name:      word,
		Svg:       s.dictionary[word].Logogram,
		Reasoning: "This is the initial svg.",
	}

	initMsgJson, err := json.Marshal(initMsg)
	if err != nil {
		return "", err
	}

	kickoff := s.cmd(agent.SendInitialMessage, string(initMsgJson))(generator)

	s.gs.Channel.ToClients <- kickoff

	for !done {
		m := <-s.gs.Channel.ToServer

		i++

		var msg *chat.Message

		// Switch the message to the recipient based on the sender. If the
		// sender is the generator, rewrite the response into one for
		// the adversary. If the sender is the adversary, rewrite the message
		// for the generator.

		switch m.Sender {
		case generator.Name:

			// Make a message for the adversary.

			var res chat.AgentLogogramIterationResponse

			err := json.Unmarshal([]byte(m.Text), &res)
			if err != nil {
				return "", errors.Wrap(err, "failed to unmarshal agent logogram iteration")
			}

			currentSvg = res.Svg

			if res.Stop {
				done = true
			}

			msg = s.cmd(
				agent.RequestLogogramCritique,
				res.Svg+"\nReasoning: "+res.Reasoning,
			)(adversary)

		case adversary.Name:

			// Make a message for the generator.

			var res chat.AgentLogogramCritiqueResponse

			err := json.Unmarshal([]byte(m.Text), &res)
			if err != nil {
				return "", errors.Wrap(err, "failed to unmarshal generator logogram critique")
			}

			if res.Stop {
				done = true
			}

			msg = s.cmd(agent.RequestLogogramIteration, res.Critique)(generator)
		}

		if done {
			continue
		}

		s.gs.Channel.ToClients <- msg

		usedWords, err := s.extractUsedWords(newGeneration.Dictionary, m.Text.String())
		if err != nil {
			return "", errors.Wrap(err, "failed finding used words")
		}

		// Broadcast the sent message.

		s.ws.Broadcasters.Messages.Broadcast(m)

		// Broadcast the extracted words from the sent message.

		s.ws.Broadcasters.MessageWordDictExtraction.Broadcast(usedWords)

		// Emit the message.
		s.logger.Debugf("Exchanges: %d", i)

		time.Sleep(10 * time.Second)
	}

	// Put everything back.

	s.resetAgents(clients)

	return currentSvg, nil
}

func (s *ConlangServer) WaitForClients(total int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		joinCtx, joinCtxCancel := context.WithCancel(ctx)

		go func() {
			joined := false
			for !joined {
				time.Sleep(1 * time.Second)
				if s.gs.TotalClients() >= total {
					joined = true
				}
			}
			joinCtxCancel()
		}()

		<-joinCtx.Done()

		s.logger.Info("All clients joined!")

		return nil
	}
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(ctx context.Context) error {
	var (
		err    error
		errMsg = "failed to evolve generation %d"
	)

	timer := utils.Timer(time.Now())

	for i := range s.config.procedures.maxGenerations {
		elapsedTime := utils.Timer(time.Now())

		genSlice, err := s.generations.ToSlice()
		if err != nil {
			return errors.Wrap(err, errMsg)
		}

		// Starts on Layer 4, recurses to 1.

		newGeneration, err := s.iterate(
			genSlice[i],
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			return errors.Wrapf(err, "failed to iterate on generation %d", i)
		}

		s.logger.Infof(
			"Iteration %d completed in %s", i+1,
			elapsedTime().Truncate(10*time.Millisecond),
		)

		updates := <-s.dictUpdatesChan

		s.logger.Info("Received dictionary update")

		// Update the generation's dictionary based on updates.

		currentDict := newGeneration.Dictionary.Copy()

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
			entry.Remove = update.Remove

			currentDict[update.Word] = entry
		}

		newGeneration.Dictionary = currentDict.Copy()

		w := "suli"

		svg, err := s.iterateLogogram(newGeneration, w)
		if err != nil {
			s.errs <- err
		}

		newGeneration.Logography[w] = svg

		err = s.generations.Enqueue(newGeneration)
		if err != nil {
			s.errs <- errors.Wrapf(
				err,
				"failed to enqueue new generation %d", i,
			)
		}

		err = s.ws.InitialData.RecentGenerations.Enqueue(newGeneration)
		if err != nil {
			s.errs <- errors.Wrapf(
				err,
				"failed to enqueue new generation to initial data %d", i,
			)
		}

		s.ws.Broadcasters.Generation.Broadcast(newGeneration)
	}

	s.gs.Listening = false

	t := timer().Truncate(10 * time.Millisecond)

	s.logger.Info("EVOLUTION COMPLETE")
	s.logger.Infof("Evolution took %s", t)

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

// outputTestData continuously outputs messages to the test API. This is
// useful for frontend testing without having to run agent queries
// over and over again.
func (s *ConlangServer) outputTestData(
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

	s.logger.Info("Emitting test output data...")

	go func() {
		for {
			<-t1.C
			s.ws.Broadcasters.TestMessageFeed.Broadcast(messages[i1%l1])
			i1++
		}
	}()

	for {
		<-t2.C
		s.ws.Broadcasters.TestGenerationsFeed.Broadcast(generations[i2%l2])
		i2++
	}
}
