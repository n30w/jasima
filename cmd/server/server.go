package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"

	"github.com/pkg/errors"

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

	config *config

	// specification are serialized versions of the Markdown specifications.
	specification chat.LayerMessageSet
}

type procedureConfig struct {
	// maxExchanges represents the total exchanges allowed per layer
	// of evolution.
	maxExchanges int

	// maxGenerations represents the maximum number of generations to evolve.
	// When set to 0, the procedure evolves forever.
	maxGenerations int

	originalSpecification chat.LayerMessageSet
	specifications        []chat.LayerMessageSet
}

type config struct {
	name         string
	debugEnabled bool
	procedures   procedureConfig
}

func NewConlangServer(
	cfg *config,
	l *log.Logger,
	m MemoryService,
	s chat.LayerMessageSet,
) *ConlangServer {
	c := channels{
		messagePool:            make(chan *chat.Message),
		systemLayerMessagePool: make(memory.MessageChannel),
		eventsMessagePool:      make(memory.MessageChannel),
		exchanged:              make(chan bool),
	}

	ct := &clientele{
		byNameMap:  make(nameToClientsMap),
		byLayerMap: make(layerToNamesMap),
		logger:     l,
	}

	if cfg.procedures.maxGenerations == 0 {
		l.Infof(
			"Max generations not specified, setting to default %d",
			DefaultMaxGenerations,
		)
		cfg.procedures.maxGenerations = DefaultMaxGenerations
	}

	return &ConlangServer{
		Server: Server{
			clients:   ct,
			name:      chat.Name(cfg.name),
			logger:    l,
			memory:    m,
			channels:  c,
			listening: true,
			messages:  make([]memory.Message, 0),
		},
		specification: s,
		config:        cfg,
	}
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
	go s.ListenAndServeRPC("tcp", "50051", errs)
}

func (s *ConlangServer) WebEvents(errs chan<- error) {
	go s.ListenAndServeWebEvents("7070", errs)
}

func (s *ConlangServer) StartProcedures(errs chan<- error) {
	go s.Evolve(errs)

	if s.config.debugEnabled {
		go func(errs chan<- error) {
			// Load test data from file JSON.
			jsonFile, err := os.Open("./outputs/chats/chat_4.json")
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

func (s *ConlangServer) Run(errs chan error) {
	s.Router(errs)
	s.WebEvents(errs)
	s.StartProcedures(errs)
}
