package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"codeberg.org/n30w/jasima/agent"

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

// generation contains all generational information related to a single
// iteration of a conlang's development.
type generation struct {
	transcript     []memory.Message
	logography     logographyGeneration
	specifications chat.LayerMessageSet
}

type ConlangServer struct {
	Server
	config          *config
	mostRecentEvent memory.Message
	webClients      map[chan memory.Message]struct{}
	procedureChan   chan memory.Message
	generations     []generation

	// specification are serialized versions of the Markdown specifications.
	specification chat.LayerMessageSet
}

type logographyGeneration map[string]string

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

type filePathConfig struct {
	specifications string
	logography     string
}

type config struct {
	name         string
	debugEnabled bool
	files        filePathConfig
	procedures   procedureConfig
}

func NewConlangServer(
	cfg *config,
	l *log.Logger,
	m MemoryService,
) (*ConlangServer, error) {
	var (
		c = channels{
			messagePool:            make(chan *chat.Message),
			systemLayerMessagePool: make(memory.MessageChannel),
			eventsMessagePool:      make(memory.MessageChannel),
			exchanged:              make(chan bool),
		}
		ct = &clientele{
			byNameMap:  make(nameToClientsMap),
			byLayerMap: make(layerToNamesMap),
			logger:     l,
		}
	)

	if cfg.procedures.maxGenerations == 0 {
		l.Infof(
			"Max generations not specified, setting to default %d",
			DefaultMaxGenerations,
		)
		cfg.procedures.maxGenerations = DefaultMaxGenerations
	}

	initialGen := generation{}

	// Load and serialize specifications.

	specifications, err := newLangSpecification(cfg.files.specifications)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize server")
	}

	initialGen.specifications = specifications

	// Load and serialize Logography SVGs.

	logographyGen1, err := loadSVGsFromDirectory(cfg.files.logography)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load logography")
	}

	initialGen.logography = logographyGen1

	generations := make([]generation, 0)
	generations = append(generations, initialGen)

	return &ConlangServer{
		Server: Server{
			clients:   ct,
			name:      chat.Name(cfg.name),
			logger:    l,
			memory:    m,
			channels:  c,
			listening: true,
		},
		webClients:    make(map[chan memory.Message]struct{}),
		generations:   generations,
		procedureChan: make(chan memory.Message),
		specification: specifications,
		config:        cfg,
	}, nil
}

func (s *ConlangServer) sendEventMessage(msg memory.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for ch := range s.webClients {
		select {
		case ch <- msg:
		default:
			s.logger.Warn("Client channel full, dropping message")
		}
	}
}

func (s *ConlangServer) Router(errs chan<- error) {
	errMsg := "failed to route message"

	eventsRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		s.mostRecentEvent = msg

		s.sendEventMessage(msg)

		return nil
	}

	printConsoleData := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		s.logger.Debugf("MESSAGE: %+v", msg)

		if msg.Command != agent.NoCommand {
			s.logger.Debugf(
				"Issued command %s to %s", msg.Command,
				msg.Receiver,
			)
		}

		if msg.Text != "" {
			// s.logger.Printf("%s: %s", msg.Sender, msg.Text)
		}

		return nil
	}

	messageRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		if msg.Layer == chat.SystemLayer && msg.Sender == chat.SystemName && msg.
			Receiver == s.name {
			s.channels.systemLayerMessagePool <- msg
			return nil
		}

		err := s.broadcast(&msg)
		if err != nil {
			return errors.Wrap(err, errMsg)
		}

		return nil
	}

	procedureRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		if msg.Sender != s.name {
			// This is necessary so the procedure channel does NOT block
			// the main router loop.
			select {
			case s.procedureChan <- msg:
				s.logger.Debug("Dispatching message to procedure channel")
			default:
			}
		}
		return nil
	}

	saveMessage := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		err := saveMessageTo(ctx, s.memory, msg)
		if err != nil {
			return errors.Wrap(err, errMsg)
		}

		return nil
	}

	routeMessages := chat.BuildRouter[chat.Message](
		s.channels.messagePool,
		printConsoleData,
		saveMessage,
		messageRoute,
		procedureRoute,
		eventsRoute,
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
			if !s.config.debugEnabled {
				go s.outputTestData(msgs)
			}
		}(errs)
	}
}

func (s *ConlangServer) Run(errs chan error) {
	s.Router(errs)
	s.WebEvents(errs)
	s.StartProcedures(errs)
}
