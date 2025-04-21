package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/commands"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/charmbracelet/log"
)

// MemoryService is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type MemoryService interface {
	// Save saves a message, using its role and text. A role of `0` saves as
	// "user". A role of `1` saves as "model".
	Save(ctx context.Context, message memory.Message) error

	// Retrieve retrieves an `n` amount of messages from the storage. An `n`
	// less-than-or-equal-to zero returns all messages. Any `n` amount
	// less-than-or-equal-to the total number of memories returns `n` messages.
	// `name` is the name of the agent that inserted the messages. This is
	// just the client name.
	Retrieve(ctx context.Context, name chat.Name, n int) (
		[]memory.Message,
		error,
	)

	Clear() error
}

type ConlangServer struct {
	Server

	// systemAgentOnline signifies whether the system LLM agent service has
	// come online or not. The evolution cannot start without it.
	systemAgentOnline bool

	// specification are serialized versions of the Markdown specifications.
	specification chat.LayerMessageSet
}

func NewConlangServer(
	name string,
	l *log.Logger,
	m LocalMemory,
	s chat.LayerMessageSet,
) *ConlangServer {
	c := channels{
		messagePool:            make(memory.MessageChannel),
		systemLayerMessagePool: make(memory.MessageChannel),
		exchanged:              make(chan bool),
	}

	ct := &clientele{
		byNameMap:  make(nameToClientsMap),
		byLayerMap: make(layerToNamesMap),
		logger:     l,
	}

	return &ConlangServer{
		Server: Server{
			clients:  ct,
			name:     chat.Name(name),
			logger:   l,
			memory:   m,
			channels: c,
		},
		specification: s,
	}
}

// iterate begins the processing of a Layer. The function completes after the
// total number of back and forth rounds are complete. Layer control and message
// routing are decoupled.
func (s *ConlangServer) iterate(
	specs []chat.Content,
	initialLayer chat.Layer,
) ([]chat.Content, error) {
	if initialLayer == chat.SystemLayer {
		return nil, nil
	}

	s.logger.Infof("RECURSED on %s", initialLayer)

	// Compile previous Layer's outputs to use in this current Layer's input

	iteration, err := s.iterate(specs[:initialLayer], initialLayer-1)
	if err != nil {
		return nil, err
	}

	newSpecs := make([]chat.Content, 0)

	if iteration != nil {
		newSpecs = append(newSpecs, specs...)
		for i, iter := range iteration {
			newSpecs[i] = iter
		}
	}

	clients := s.getClientsByLayer(initialLayer)

	s.logger.Infof("Sending %s to %s", commands.Unlatch, initialLayer)

	for _, v := range clients {

		// First append new instructions to clients.
		// Send prevSpec to clients. Compile specs into a single system instruction

		content := s.specification[initialLayer]

		s.channels.messagePool <- *s.newCommand(
			v,
			commands.AppendInstructions, content,
		)

		// Then unlatch them... They're ready.

		s.channels.messagePool <- *s.newCommand(v, commands.Unlatch)
	}

	var initMsg chat.Content = "Hello, let's begin."

	// Select the first client in the layer to be the initializer.

	initializerClient := s.getClientsByLayer(initialLayer)[0]

	s.channels.messagePool <- *s.newCommand(
		initializerClient,
		commands.SendInitialMessage, initMsg,
	)

	// Dispatch iterate commands to clients on Layer.

	exchanges := 10

	for i := range exchanges {
		<-s.channels.exchanged
		s.logger.Infof("Exchange Total: %d", i+1)
	}

	// Send every client in the Layer clear memory command.

	err = s.sendCommands(clients, commands.Latch, commands.ClearMemory)
	if err != nil {
		return nil, err
	}

	sysClient := s.getClientsByLayer(chat.SystemLayer)[0]

	add := chat.Content(fmt.Sprintf("You are responsible for developing: %s", initialLayer))

	s.channels.messagePool <- *s.newCommand(
		sysClient,
		commands.AppendInstructions,
		add+s.specification[initialLayer],
	)
	s.channels.messagePool <- *s.newCommand(sysClient, commands.Unlatch)

	text := chat.Content(s.memory.String())

	msg := &memory.Message{
		Sender:   s.name,
		Receiver: "",
		Layer:    chat.SystemLayer,
		Text:     text,
	}

	s.channels.messagePool <- *msg

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.
	s.logger.Infof("Waiting for systemLayerMessagePool...")
	specPrime := <-s.channels.systemLayerMessagePool
	if iteration != nil {
		newSpecs[initialLayer] = specPrime.Text
	} else {
		newSpecs = append(newSpecs, specPrime.Text)
	}
	s.channels.messagePool <- *s.newCommand(sysClient, commands.Latch)

	return newSpecs, nil
}

func (s *ConlangServer) newCommand(
	c *client,
	command commands.Command, content ...chat.Content,
) *memory.Message {
	msg := &memory.Message{
		Sender:   s.name,
		Receiver: c.name,
		Command:  command,
		Layer:    c.layer,
		Text:     "",
	}

	if len(content) > 0 {
		msg.Text = content[0]
	}

	return msg
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
		specs, err = s.iterate(specs, chat.LogographyLayer)
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

func (s *ConlangServer) sendCommands(
	clients []*client,
	commands ...commands.Command,
) error {
	var err error

	for _, v := range clients {
		for _, cmd := range commands {
			err = s.sendCommand(cmd, v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ConlangServer) Run(errs chan error) {
	go s.ListenAndServe(errs)
	go s.router()
	go s.Evolve(errs)
}

func NewLangSpecification(p string) (chat.LayerMessageSet, error) {
	ls := make(chat.LayerMessageSet)

	b, err := os.ReadFile(filepath.Join(p, "dictionary.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.DictionaryLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "grammar.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.GrammarLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "logography.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.LogographyLayer] = chat.Content(b)

	b, err = os.ReadFile(filepath.Join(p, "phonetics.md"))
	if err != nil {
		return nil, err
	}

	ls[chat.PhoneticsLayer] = chat.Content(b)

	return ls, nil
}
