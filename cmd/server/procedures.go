package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/agent"
	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
	"codeberg.org/n30w/jasima/utils"
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

	sysClient, err := s.getClientByName(chat.Name("SYSTEM_AGENT_A"))
	if err != nil {
		return newGeneration, errors.Wrap(
			err,
			"failed to retrieve client by name",
		)
	}

	// layerSpecificInstructions := map[chat.Layer]string{
	// 	chat.SystemLayer:     "",
	// 	chat.PhoneticsLayer:  "",
	// 	chat.GrammarLayer:    "",
	// 	chat.DictionaryLayer: "Consider words based on this text:",
	// 	chat.LogographyLayer: "",
	// }

	var (
		timer        = utils.Timer(time.Now())
		clients      = s.getClientsByLayer(initialLayer)
		cmd          = buildCommand(s.config.name)
		sendCommands = sendCommandBuilder(s.channels.messagePool)

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
		)(s.getClientsByLayer(initialLayer)[0])

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

		sb strings.Builder
	)

	newGeneration.Transcript = prevGeneration.Transcript.Copy()
	newGeneration.Specifications = prevGeneration.Specifications.Copy()

	sb.WriteString(initialInstructions)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newGeneration.Specifications[i].String())
		sb.WriteString("\n")
	}

	sendCommands(
		clients,
		cmd(agent.AppendInstructions, sb.String()),
		cmd(agent.Unlatch),
	)

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	s.channels.messagePool <- kickoff

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
	s.channels.messagePool <- addSysAgentInstructions
	s.channels.messagePool <- cmd(agent.Unlatch)(sysClient)

	msg := s.messageToSystemAgent(
		sysClient.name,
		transcriptToString(newGeneration.Transcript[initialLayer]),
	)

	s.channels.messagePool <- msg

	s.logger.Infof("Waiting for system agent response...")

	specPrime := <-s.channels.systemLayerMessagePool

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

	return newGeneration, nil
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(errs chan<- error) {
	var (
		err         error
		errMsg      = "failed to evolve generation %d"
		targetTotal = 10
		cmd         = buildCommand(s.config.name)
	)

	joinCtx, joinCtxCancel := context.WithCancel(context.Background())

	go func() {
		joined := false
		for !joined {
			time.Sleep(1 * time.Second)
			if s.clients.total >= targetTotal {
				joined = true
			}
		}
		joinCtxCancel()
	}()

	<-joinCtx.Done()

	s.logger.Info("All clients joined!")

	timer := utils.Timer(time.Now())

	// Prepare the dictionary system agent.

	dictSysAgent, err := s.getClientByName(chat.Name("SYSTEM_AGENT_B"))
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

		s.channels.messagePool <- cmd(agent.Latch)(dictSysAgent)
		s.channels.messagePool <- cmd(
			agent.AppendInstructions,
			fmt.Sprintf(
				"This is the current dictionary\n%s",
				genSlice[i].Dictionary.String(),
			),
		)(dictSysAgent)

		// Starts on Layer 4, recurses to 1.

		newGeneration, err := s.iterate(
			genSlice[i],
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			errs <- errors.Wrapf(err, errMsg, i)
			return
		}

		s.logger.Infof(
			"Iteration %d completed in %s", i+1,
			elapsedTime().Truncate(10*time.Millisecond),
		)

		// send results to dictionary LLM

		s.channels.messagePool <- cmd(agent.Unlatch)(dictSysAgent)

		msg := s.messageToSystemAgent(
			dictSysAgent.name,
			transcriptToString(newGeneration.Transcript[chat.DictionaryLayer]),
		)

		s.channels.messagePool <- msg

		// Unmarshal from JSON.

		var updates []memory.DictionaryEntry

		noErrorJson := false

		for !noErrorJson {
			dictUpdates := <-s.channels.systemLayerMessagePool
			err = json.Unmarshal([]byte(dictUpdates.Text), &updates)
			if err != nil {
				s.logger.Errorf("failed to unmarshal dictionary update generation %d", i)
				s.logger.Debug("Resending dictionary request to system agent...")
				msg := s.messageToSystemAgent(
					dictSysAgent.name,
					"Your JSON is imporperly formatted. Please send a corrected version.",
				)
				s.channels.messagePool <- msg
				continue
			}
			noErrorJson = true
		}

		// Update the generation's dictionary based on updates.

		for _, update := range updates {
			if update.Remove {
				_, ok := newGeneration.Dictionary[update.Word]
				if !ok {
					s.logger.Warnf(
						"%s not in dictionary, skipping",
						update.Word,
					)
					continue
				}

				delete(newGeneration.Dictionary, update.Word)
				continue
			}

			newGeneration.Dictionary[update.Word] = update
		}

		s.channels.messagePool <- cmd(agent.Latch)(dictSysAgent)
		s.channels.messagePool <- cmd(agent.ClearMemory)(dictSysAgent)
		s.channels.messagePool <- cmd(agent.ResetInstructions)(dictSysAgent)

		err = s.generations.Enqueue(newGeneration)
		if err != nil {
			errs <- errors.Wrapf(err, "failed to enqueue new generation %d", i)
		}

		// Save specs to memory
		// Save result to LLM.
		s.broadcasters.generation.Broadcast(newGeneration)
	}

	s.listening = false

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
func (s *ConlangServer) outputTestData(data []memory.Message) {
	var (
		i = 0
		l = len(data)
		t = time.NewTicker(time.Second)
	)

	s.logger.Info("Emitting test output data...")

	for {
		<-t.C
		s.broadcasters.testMessageFeed.Broadcast(data[i%l])
		i++
	}
}
