package main

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/n30w/jasima/agent"

	"codeberg.org/n30w/jasima/utils"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
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

	// Add 1 so that we can fit a new spec document into the specification.
	// Instead of making a new variable `newSpecs`, one could easily
	// just use the already defined `specs`.

	newSpecs := make([]chat.Content, 0, initialLayer+1)

	// Make a new specification from the original specification.

	newSpecs = append(newSpecs, specs...)

	// Then modify that specification with the previous layer's specification.

	copy(newSpecs, iteration)

	// Start of side effects.

	clients := s.getClientsByLayer(initialLayer)

	var sb strings.Builder

	sb.WriteString(
		fmt.Sprintf(
			"You and your interlocutors are responsible for developing %s."+
				"\nHere is the current specification.",
			initialLayer,
		),
	)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newSpecs[i].String())
	}

	content := chat.Content(sb.String())

	s.logger.Infof("Sending %s to %s", agent.Unlatch, initialLayer)

	for _, v := range clients {
		s.channels.messagePool <- *s.newCommand(
			v,
			agent.AppendInstructions,
			content,
		)
		s.channels.messagePool <- *s.newCommand(v, agent.Unlatch)
	}

	initMsg := chat.Content(
		fmt.Sprintf(
			"Hello, "+
				"let's begin developing Toki Pona %s. You go first.",
			initialLayer,
		),
	)

	// Select the first client in the layer to be the initializer.

	initializerClient := s.getClientsByLayer(initialLayer)[0]

	s.channels.messagePool <- *s.newCommand(
		initializerClient,
		agent.SendInitialMessage,
		initMsg,
	)

	// Dispatch iterate commands to clients on Layer.

	for i := range exchanges {
		<-s.channels.exchanged
		s.logger.Infof("Exchange Total: %d/%d", i+1, exchanges)
	}

	err = s.sendCommands(
		clients,
		agent.Latch,
		agent.ClearMemory,
		agent.ResetInstructions,
	)
	if err != nil {
		return nil, err
	}

	sysClient := s.getClientsByLayer(chat.SystemLayer)[0]

	sb.Reset()

	// The system agent will ONLY summarize the chat log, and not read the
	// other specifications for other layers (for now at least).

	sb.WriteString(
		fmt.Sprintf(
			"The current specification is for: %s. "+
				"Here is the current specification:\n",
			initialLayer,
		),
	)

	sb.WriteString(specs[initialLayer].String())

	s.channels.messagePool <- *s.newCommand(
		sysClient,
		agent.AppendInstructions,
		chat.Content(sb.String()),
	)

	s.channels.messagePool <- *s.newCommand(sysClient, agent.Unlatch)

	sb.Reset()

	sb.WriteString("=== BEGIN CHAT LOG ===\n")
	sb.WriteString(s.memory.String())
	sb.WriteString("\n=== END CHAT LOG ===")

	msg := &memory.Message{
		Sender:   s.name,
		Receiver: "",
		Layer:    chat.SystemLayer,
		Text:     chat.Content(sb.String()),
	}

	s.channels.messagePool <- *msg

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.
	s.logger.Infof("Waiting for systemLayerMessagePool...")

	specPrime := <-s.channels.systemLayerMessagePool

	sb.Reset()

	s.channels.messagePool <- *s.newCommand(sysClient, agent.Latch)
	s.channels.messagePool <- *s.newCommand(sysClient, agent.ClearMemory)

	// End of side effects.

	newSpecs = append(newSpecs, specPrime.Text)

	return newSpecs, nil
}

// Evolve manages the entire evolutionary function loop.
func (s *ConlangServer) Evolve(errs chan<- error) {
	var err error

	targetTotal := 9

	allJoined := make(chan struct{})

	go func(c chan<- struct{}) {
		joined := false
		for !joined {
			time.Sleep(1 * time.Second)
			s.mu.Lock()
			v := s.clients.byNameMap
			if len(v) >= targetTotal {
				joined = true
			}
			s.mu.Unlock()
		}
		close(c)
	}(allJoined)

	<-allJoined

	s.logger.Info("all clients joined.")

	specs := s.specification.ToSlice()

	for range 1 {
		elapsedTime := utils.Timer(time.Now())
		// Starts on Layer 4, recurses to 1.
		specs, err = s.iterate(specs, chat.LogographyLayer, s.exchangeTotal)
		if err != nil {
			errs <- err
			return
		}
		s.logger.Infof(
			"Iteration %d completed in %s", targetTotal,
			elapsedTime(),
		)
		// Save specs to memory
		// send results to SYSTEM LLM
		// Save result to LLM.
	}

	s.logger.Info("EVOLUTION COMPLETE")
}
