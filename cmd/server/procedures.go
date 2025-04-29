package main

import (
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
	initialGeneration generation,
	initialLayer chat.Layer,
	exchanges int,
) (generation, error) {
	if initialLayer == chat.SystemLayer {
		return generation{
			transcript:     make([]memory.Message, 0),
			logography:     initialGeneration.logography,
			specifications: initialGeneration.specifications,
		}, nil
	}

	s.logger.Infof("%d: Recursing on %s", initialLayer, initialLayer)

	prevGeneration, err := s.iterate(
		initialGeneration, initialLayer-1,
		exchanges,
	)
	if err != nil {
		return initialGeneration, errors.Wrapf(
			err,
			"failed iteration at %s",
			initialLayer,
		)
	}

	var (
		timer        = utils.Timer(time.Now())
		clients      = s.getClientsByLayer(initialLayer)
		sysClient    = s.getClientsByLayer(chat.SystemLayer)[0]
		cmd          = buildCommand(s.config.name)
		sendCommands = sendCommandBuilder(s.channels.messagePool)
		transcript   = make([]memory.Message, 0)

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
				initialLayer,
				initialGeneration.specifications[initialLayer].String(),
			),
		)(sysClient)

		sb strings.Builder
	)

	newGeneration := generation{
		// transcript currently does NOT save the previous chat's transcript
		// due to rate limit issues. Gotta fix this.
		transcript:     make([]memory.Message, 0),
		logography:     prevGeneration.logography,
		specifications: prevGeneration.specifications,
	}

	sb.WriteString(initialInstructions)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newGeneration.specifications[i].String())
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
		m := <-s.procedureChan
		transcript = append(transcript, m)
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
		s.name,
		sysClient.name,
		chat.Content(transcriptToString(transcript)),
		chat.SystemLayer,
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
		timer().Truncate(1*time.Millisecond),
	)

	// End of side effects.

	newGeneration.specifications[initialLayer] = specPrime.Text
	newGeneration.transcript = append(newGeneration.transcript, transcript...)

	return newGeneration, nil
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(errs chan<- error) {
	var (
		err         error
		targetTotal = 9
		allJoined   = make(chan struct{})
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
		newGeneration, err := s.iterate(
			s.generations[i],
			chat.LogographyLayer,
			s.config.procedures.maxExchanges,
		)
		if err != nil {
			errs <- errors.Wrapf(err, "failed to evolve generation %d", i)
			return
		}

		s.logger.Infof(
			"Iteration %d completed in %s", i+1,
			elapsedTime().Truncate(10*time.Millisecond),
		)

		s.generations = append(s.generations, newGeneration)

		// Save specs to memory
		// send results to SYSTEM LLM
		// Save result to LLM.
		s.broadcasters.generation.Broadcast(newGeneration)
	}

	s.listening = false

	s.logger.Info("EVOLUTION COMPLETE")

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
	err = saveToJson(s.generations, fileName)
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
