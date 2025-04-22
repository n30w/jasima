package server

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/commands"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"
)

// iterate begins the processing of a Layer. The function completes after the
// total number of back and forth rounds are complete. Layer control and message
// routing are decoupled.
func (s *ConlangServer) iterate(
	specs []chat.Content,
	initialLayer chat.Layer,
	exchanges int,
) ([]chat.Content, error) {
	newSpecs := make([]chat.Content, initialLayer+1)

	if initialLayer == chat.SystemLayer {
		return newSpecs, nil
	}

	s.logger.Infof("%d: Recursing on %s", initialLayer, initialLayer)

	// Compile previous Layer's outputs to use in this current Layer's input.

	nextLayer := initialLayer - 1

	iteration, err := s.iterate(specs[:nextLayer], nextLayer, exchanges)
	if err != nil {
		return nil, err
	}

	if len(iteration) > 0 {
		newSpecs = append(newSpecs, specs...)
		copy(newSpecs, specs)
	}

	clients := s.getClientsByLayer(initialLayer)

	s.logger.Infof("Sending %s to %s", commands.Unlatch, initialLayer)

	var sb strings.Builder

	sb.WriteString(
		fmt.Sprintf(
			"You and your interlocutors are responsible for developing %s \nHere is the current specification.",
			initialLayer,
		),
	)

	for i := initialLayer; i > 0; i-- {
		sb.WriteString(newSpecs[i].String())
	}

	content := chat.Content(sb.String())

	for _, v := range clients {
		s.channels.messagePool <- *s.newCommand(
			v,
			commands.AppendInstructions,
			content,
		)
		s.channels.messagePool <- *s.newCommand(v, commands.Unlatch)
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
		commands.SendInitialMessage,
		initMsg,
	)

	// Dispatch iterate commands to clients on Layer.

	for i := range exchanges {
		<-s.channels.exchanged
		s.logger.Infof("Exchange Total: %d/%d", i+1, exchanges)
	}

	err = s.sendCommands(clients, commands.Latch, commands.ClearMemory)
	if err != nil {
		return nil, err
	}

	sysClient := s.getClientsByLayer(chat.SystemLayer)[0]

	sb.Reset()
	sb.WriteString(
		fmt.Sprintf(
			"You are responsible for developing: %s \nHere is the current specification.",
			initialLayer,
		),
	)

	// Use the current spec so the LLM can compare to the chat logs.

	sb.WriteString(specs[initialLayer].String())

	s.channels.messagePool <- *s.newCommand(
		sysClient,
		commands.AppendInstructions,
		chat.Content(sb.String()),
	)

	s.channels.messagePool <- *s.newCommand(sysClient, commands.Unlatch)

	chatLog := chat.Content(s.memory.String())

	msg := &memory.Message{
		Sender:   s.name,
		Receiver: "",
		Layer:    chat.SystemLayer,
		Text:     chatLog,
	}

	s.channels.messagePool <- *msg

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.
	s.logger.Infof("Waiting for systemLayerMessagePool...")

	specPrime := <-s.channels.systemLayerMessagePool

	newSpecs[initialLayer] = specPrime.Text
	// newSpecs = append(newSpecs, specPrime.Text)

	s.channels.messagePool <- *s.newCommand(sysClient, commands.Latch)

	s.channels.messagePool <- *s.newCommand(sysClient, commands.ClearMemory)

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
		// Starts on Layer 4, recurses to 1.
		specs, err = s.iterate(specs, chat.LogographyLayer, s.exchangeTotal)
		if err != nil {
			errs <- err
			return
		}
		// Save specs to memory
		// send results to SYSTEM LLM
		// Save result to LLM.
	}

	s.logger.Info("EVOLUTION COMPLETE")
}
