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
		timer        = utils.Timer(time.Now())
		clients      = s.gs.GetClientsByLayer(initialLayer)
		cmd          = network.BuildCommand(s.config.name)
		sendCommands = network.SendCommandBuilder(s.gs.Channel.ToClients)

		// kickoff is the command that is sent to kick off the layer's
		// iteration exchanges. It consists of the initializer client, which
		// is the first client of the layer, and content, which is the text
		// in the message that the initializer client will send to other
		// clients on the layer.
		kickoff = cmd(
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
		addSysAgentInstructions = cmd(
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

	sendCommands(
		clients,
		cmd(agent.AppendInstructions, sb.String()),
		cmd(agent.Unlatch),
	)

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	s.gs.Channel.ToClients <- kickoff

	for i := range exchanges {
		m := <-s.procedureChan
		newGeneration.Transcript[initialLayer] = append(
			newGeneration.Transcript[initialLayer],
			m,
		)
		s.logger.Infof("Exchange Total: %d/%d", i+1, exchanges)
	}

	sendCommands(
		clients,
		cmd(agent.Latch),
		cmd(agent.ClearMemory),
		cmd(agent.ResetInstructions),
	)

	sb.Reset()

	// The system agent will ONLY summarize the chat log, and not read the
	// other Specifications for other layers (for now at least).
	s.gs.Channel.ToClients <- addSysAgentInstructions
	s.gs.Channel.ToClients <- cmd(agent.Unlatch)(sysClient)

	msg := s.messageToSystemAgent(
		sysClient.Name,
		transcriptToString(newGeneration.Transcript[initialLayer]),
	)

	s.gs.Channel.ToClients <- msg

	s.logger.Infof("Waiting for system agent response...")

	specPrime := <-s.gs.Channel.ToServer

	sb.Reset()

	sendCommands(
		clients,
		cmd(agent.Latch),
		cmd(agent.ClearMemory),
		cmd(agent.ResetInstructions),
	)

	s.logger.Infof(
		"%s took %s to complete",
		initialLayer,
		timer().Truncate(time.Millisecond),
	)

	// End of side effects.

	newGeneration.Specifications[initialLayer] = specPrime.Text

	// JK one more.

	s.ws.Broadcasters.Specification.Broadcast(
		newGeneration.
			Specifications,
	)

	return newGeneration, nil
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(errs chan<- error) {
	var (
		err         error
		errMsg      = "failed to evolve generation %d"
		targetTotal = 10
		cmd         = network.BuildCommand(s.config.name)
	)

	joinCtx, joinCtxCancel := context.WithCancel(context.Background())

	go func() {
		joined := false
		for !joined {
			time.Sleep(1 * time.Second)
			if s.gs.TotalClients() >= targetTotal {
				joined = true
			}
		}
		joinCtxCancel()
	}()

	<-joinCtx.Done()

	s.logger.Info("All clients joined!")

	timer := utils.Timer(time.Now())

	// Prepare the dictionary system agent.

	dictSysAgent, err := s.gs.GetClientByName("SYSTEM_AGENT_B")
	if err != nil {
		errs <- err
		return
	}

	for i := range s.config.procedures.maxGenerations {
		elapsedTime := utils.Timer(time.Now())

		genSlice, err := s.generations.ToSlice()
		if err != nil {
			errs <- errors.Wrap(err, errMsg)
			return
		}

		genDict, err := json.Marshal(genSlice[i].Dictionary)
		if err != nil {
			errs <- errors.Wrap(err, "failed to Marshal dictionary")
			return
		}

		s.gs.Channel.ToClients <- cmd(agent.Latch)(dictSysAgent)
		s.gs.Channel.ToClients <- cmd(
			agent.AppendInstructions,
			fmt.Sprintf(
				"This is the current dictionary\n%s",
				string(genDict),
			),
		)(dictSysAgent)

		// Starts on Layer 4, recurses to 1.

		newGeneration, err := s.iterate(
			genSlice[i],
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			errs <- errors.Wrapf(err, "failed to iterate on generation %d", i)
			return
		}

		s.logger.Infof(
			"Iteration %d completed in %s", i+1,
			elapsedTime().Truncate(10*time.Millisecond),
		)

		// send results to dictionary LLM

		s.gs.Channel.ToClients <- cmd(agent.Unlatch)(dictSysAgent)

		msg := s.messageToSystemAgent(
			dictSysAgent.Name,
			transcriptToString(newGeneration.Transcript[chat.DictionaryLayer]),
		)

		s.gs.Channel.ToClients <- msg

		// Unmarshal from JSON.

		var updates []memory.DictionaryEntry

		noErrorJson := false

		for !noErrorJson {
			dictUpdates := <-s.gs.Channel.ToServer
			err = json.Unmarshal([]byte(dictUpdates.Text), &updates)
			if err != nil {
				s.logger.Errorf(
					"failed to unmarshal dictionary update generation %d",
					i,
				)
				s.logger.Debug("Resending dictionary request to system agent...")
				msg := s.messageToSystemAgent(
					dictSysAgent.Name,
					"Your JSON is improperly formatted. Please send a corrected version.",
				)
				s.gs.Channel.ToClients <- msg
				continue
			}
			noErrorJson = true
		}

		// Update the generation's dictionary based on updates.

		currentDict := newGeneration.Dictionary.Copy()

		for _, update := range updates {
			currentDict[update.Word] = update

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
			}
		}

		newGeneration.Dictionary = currentDict.Copy()

		s.gs.Channel.ToClients <- cmd(agent.Latch)(dictSysAgent)
		s.gs.Channel.ToClients <- cmd(agent.ClearMemory)(dictSysAgent)
		s.gs.Channel.ToClients <- cmd(agent.ResetInstructions)(dictSysAgent)

		err = s.generations.Enqueue(newGeneration)
		if err != nil {
			errs <- errors.Wrapf(err, "failed to enqueue new generation %d", i)
		}

		// Save specs to memory
		// Save result to LLM.
		s.ws.Broadcasters.Generation.Broadcast(newGeneration)
	}

	s.gs.Listening = false

	t := timer().Truncate(10 * time.Millisecond)

	s.logger.Info("EVOLUTION COMPLETE")
	s.logger.Infof("Evolution took %s", t)

	// Marshal to JSON

	allMsgs, err := s.memory.All()
	if err != nil {
		errs <- err
		return
	}

	fileName := fmt.Sprintf(
		"./outputs/chats/chat_%s.json",
		time.Now().Format("20060102150405"),
	)
	err = saveToJson(allMsgs, fileName)
	if err != nil {
		errs <- errors.Wrap(err, "evolution failed to save JSON")
		return
	}

	s.logger.Infof("Saved chat to %s", fileName)

	fileName = fmt.Sprintf(
		"./outputs/generations/generations_%s.json",
		time.Now().Format("20060102150405"),
	)

	g, err := s.generations.ToSlice()
	if err != nil {
		errs <- errors.Wrap(err, "evolution failed to save JSON")
	}

	err = saveToJson(g, fileName)
	if err != nil {
		errs <- errors.Wrap(err, "evolution failed to save JSON")
		return
	}

	s.logger.Infof("Saved generations to %s", fileName)
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
