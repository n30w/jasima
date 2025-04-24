package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/n30w/jasima/agent"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"

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

	fmt.Stringer
}

type ConlangServer struct {
	Server

	// specification are serialized versions of the Markdown specifications.
	specification chat.LayerMessageSet

	// exchangeTotal represents the maximum number of exchanges between agents
	// per layer.
	exchangeTotal int
}

func NewConlangServer(
	name string,
	l *log.Logger,
	m MemoryService,
	s chat.LayerMessageSet,
	e int,
) *ConlangServer {
	c := channels{
		messagePool:            make(memory.MessageChannel),
		systemLayerMessagePool: make(memory.MessageChannel),
		eventsMessagePool:      make(memory.MessageChannel),
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
		exchangeTotal: e,
	}
}

func (s *ConlangServer) newCommand(
	c *client,
	command agent.Command, content ...chat.Content,
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

func (s *ConlangServer) sendCommands(
	clients []*client,
	commands ...agent.Command,
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
	go s.ListenAndServeRouter(errs)
	go s.router()
	go s.Evolve(errs)
	go s.ListenAndServeWebEvents(errs)
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
