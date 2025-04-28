package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

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

	// All retrieves all memories.
	All() ([]memory.Message, error)

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
		messagePool:            make(chan chat.Message),
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
			clients:   ct,
			name:      chat.Name(name),
			logger:    l,
			memory:    m,
			channels:  c,
			listening: true,
			messages:  make([]memory.Message, 0),
		},
		specification: s,
		exchangeTotal: e,
	}
}

func (s *ConlangServer) newCommand(
	c *client,
	command agent.Command, content ...chat.Content,
) *chat.Message {
	msg := &chat.Message{
		Sender:   s.name.String(),
		Receiver: c.name.String(),
		Command:  command.Int32(),
		Layer:    c.layer.Int32(),
		Content:  "",
	}

	if len(content) > 0 {
		msg.Content = string(content[0])
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

			// Sleep in between commands so that agents can breathe.

			time.Sleep(time.Millisecond * 200)
		}
	}

	return nil
}

func (s *ConlangServer) Router(errs chan<- error) {
	eventsRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		select {
		case s.channels.eventsMessagePool <- msg:
			s.logger.Debug("Emitted event message")
		default:
		}

		return nil
	}

	messageRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		errMsg := "failed to route message"

		// Convert pbMsg into domain type

		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		if msg.Sender == s.name {
			s.logger.Debugf(
				"Issued command %s to %s", msg.Command,
				msg.Receiver,
			)
		}

		s.messages = append(s.messages, msg)

		err := saveMessageTo(ctx, s.memory, msg)
		if err != nil {
			return errors.Wrap(err, errMsg)
		}

		if msg.Text != "" {
			s.logger.Printf("%s: %s", msg.Sender, msg.Text)
		}

		// Route messages for the server that come from the system layer
		// agents.

		if msg.Layer == chat.SystemLayer && msg.Sender == chat.SystemName && msg.
			Receiver == s.name {
			s.channels.systemLayerMessagePool <- msg
			return nil
		}

		// If the message is not from the server itself, save it to memory
		// and notify that an exchange has occurred.

		if msg.Sender != s.name {
			err = saveMessageTo(ctx, s.memory, msg)
			if err != nil {
				return errors.Wrap(err, errMsg)
			}

			select {
			case s.channels.exchanged <- true:
			default:
			}
		}

		err = s.broadcast(&msg)
		if err != nil {
			return errors.Wrap(err, errMsg)
		}

		return nil
	}

	routeMessages := chat.BuildRouter[chat.Message](
		s.channels.messagePool,
		eventsRoute,
		messageRoute,
	)

	go routeMessages(errs)
}

func (s *ConlangServer) Run(errs chan error, debug bool) {
	s.Router(errs)
	go s.ListenAndServeRouter(errs)
	go s.Evolve(errs)
	go s.ListenAndServeWebEvents(errs)

	if debug {
		go func(errs chan error) {
			// Load test data from file JSON.
			jsonFile, err := os.Open("./outputs/chats/chat_2.json")
			if err != nil {
				errs <- err
				return
			}

			defer jsonFile.Close()

			b, _ := io.ReadAll(jsonFile)

			var msgs []memory.Message

			err = json.Unmarshal(b, &msgs)
			if err != nil {
				errs <- err
				return
			}

			// Output test data to channel.
			go s.outputTestData(msgs)
		}(errs)
	}
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
