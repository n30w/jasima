package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	specs []chat.Content,
	initialLayer chat.Layer,
	exchanges int,
) ([]chat.Content, error) {
	if initialLayer == chat.SystemLayer {
		return specs, nil
	}

	s.logger.Infof("%d: Recursing on %s", initialLayer, initialLayer)

	iteration, err := s.iterate(specs, initialLayer-1, exchanges)
	if err != nil {
		return nil, err
	}

	var (
		timer        = utils.Timer(time.Now())
		clients      = s.getClientsByLayer(initialLayer)
		sysClient    = s.getClientsByLayer(chat.SystemLayer)[0]
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
			"You and your interlocutors are responsible for developing %s."+
				"\nHere is the current specification.\n",
			initialLayer,
		)

		// addSysAgentInstructions is a command to append the current
		// specification to the system agent(s).
		addSysAgentInstructions = cmd(
			agent.AppendInstructions,
			fmt.Sprintf(
				"The current specification is for: %s. "+
					"Here is the current specification:\n%s",
				initialLayer, specs[initialLayer].String(),
			),
		)(sysClient)

		// newSpecs is the new specification that will have been developed by
		// the end of this iteration. `1` is added because the new document
		// needs to be fit into the specification, because the system layer is
		// at index 0.
		newSpecs = make([]chat.Content, 0, initialLayer+1)

		sb strings.Builder
	)

	// Make a new specification from the original specification.

	newSpecs = append(newSpecs, specs...)

	// Then modify that specification with the previous layer's specification.

	copy(newSpecs, iteration)

	sb.WriteString(initialInstructions)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newSpecs[i].String())
		sb.WriteString("\n")
	}

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	sendCommands(
		clients,
		cmd(agent.AppendInstructions, sb.String()),
		cmd(agent.Unlatch),
	)

	s.channels.messagePool <- kickoff

	for i := range exchanges {
		<-s.channels.exchanged
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
	// other specifications for other layers (for now at least).
	s.channels.messagePool <- addSysAgentInstructions
	s.channels.messagePool <- cmd(agent.Unlatch)(sysClient)

	msg := chat.NewPbMessage(
		s.name, "SYSTEM",
		chat.Content(memoryToString(s.memory)), chat.SystemLayer,
	)

	s.channels.messagePool <- msg

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.
	s.logger.Infof("Waiting for system agent response...")

	specPrime := <-s.channels.systemLayerMessagePool

	sb.Reset()

	sendCommands(
		clients,
		cmd(agent.Latch),
		cmd(agent.ClearMemory),
		cmd(agent.ResetInstructions),
	)

	err = s.memory.Clear()
	if err != nil {
		return nil, errors.Wrap(err, "failed to clear memory")
	}

	s.logger.Infof(
		"%s took %s to complete",
		initialLayer,
		timer().Truncate(1*time.Millisecond),
	)

	// End of side effects.

	newSpecs[initialLayer] = specPrime.Text

	return newSpecs, nil
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(errs chan<- error) {
	var (
		err         error
		targetTotal = 9
		allJoined   = make(chan struct{})
		specs       = s.specification.ToSlice()
		generations = s.config.procedures.maxGenerations
	)

	go func(c chan<- struct{}) {
		joined := false
		for !joined {
			time.Sleep(1 * time.Second)
			if s.clients.total >= targetTotal {
				joined = true
			}
		}
		close(c)
	}(allJoined)

	<-allJoined

	s.logger.Info("All clients joined!")

	for i := range generations {
		elapsedTime := utils.Timer(time.Now())
		// Starts on Layer 4, recurses to 1.
		specs, err = s.iterate(
			specs,
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			errs <- err
			return
		}

		s.logger.Infof(
			"Iteration %d completed in %s", i+1,
			elapsedTime().Truncate(10*time.Millisecond),
		)

		// Save specs to memory
		// send results to SYSTEM LLM
		// Save result to LLM.
	}

	s.listening = false

	s.logger.Info("EVOLUTION COMPLETE")

	// Marshal to JSON

	data, err := json.MarshalIndent(s.messages, "", "  ")
	if err != nil {
		errs <- err
		return
	}

	// Write to file

	fileName := fmt.Sprintf(
		"./outputs/chats/chat_%s.json",
		time.Now().Format("20060102150405"),
	)

	err = os.WriteFile(fileName, data, 0o644)
	if err != nil {
		errs <- err
		return
	}
}

// outputTestData continuously outputs messages to the test API. This is
// useful for frontend testing without having to run agent queries
// over and over again.
func (s *ConlangServer) outputTestData(data []memory.Message) {
	var (
		waitTime = time.Millisecond * 2000
		i        = 0
		l        = len(data)
	)

	for {
		// Send message to channel.
		select {
		case s.channels.eventsMessagePool <- data[i%l]:
		default:
		}
		time.Sleep(waitTime)
		i++
	}
}
