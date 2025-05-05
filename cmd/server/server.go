package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/network"
	"codeberg.org/n30w/jasima/pkg/utils"

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
	logger        *log.Logger
	memory        MemoryService
	gs            *network.GRPCServer
	config        *config
	procedureChan chan memory.Message
	generations   *utils.FixedQueue[memory.Generation]
	ws            *network.WebServer
}

func NewConlangServer(
	cfg *config,
	l *log.Logger,
	m MemoryService,
) (*ConlangServer, error) {
	if cfg.procedures.maxGenerations == 0 {
		l.Infof(
			"Max generations not specified, setting to default %d",
			DefaultMaxGenerations,
		)
		cfg.procedures.maxGenerations = DefaultMaxGenerations
	}

	transcriptGen1 := newTranscriptGeneration()

	// Load and serialize specifications.

	specificationsGen1, err := loadSpecificationsFromFile(
		cfg.files.
			specifications,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading specifications from file")
	}

	// Load and serialize logography SVGs.

	logographyGen1, err := loadLogographySvgsFromFile(cfg.files.logography)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading logography from files")
	}

	// Load and serialize dictionary.

	dictionaryGen1, err := loadDictionaryFromFile(cfg.files.dictionary)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading dictionary from file")
	}

	initialGen := memory.Generation{
		Transcript:     transcriptGen1,
		Logography:     logographyGen1,
		Specifications: specificationsGen1,
		Dictionary:     dictionaryGen1,
	}

	generations, err := utils.NewFixedQueue[memory.Generation](
		cfg.procedures.
			maxGenerations + 1,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create generation queue")
	}

	err = generations.Enqueue(initialGen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to enqueue initial generation")
	}

	webServer, err := network.NewWebServer(l)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create web server")
	}

	grpcServer := network.NewGRPCServer(l, cfg.name)

	err = webServer.InitialData.RecentSpecifications.Enqueue(specificationsGen1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to enqueue specifications")
	}

	return &ConlangServer{
		memory:        m,
		gs:            grpcServer,
		ws:            webServer,
		generations:   generations,
		procedureChan: make(chan memory.Message),
		config:        cfg,
		logger:        l,
	}, nil
}

func (s *ConlangServer) Router(errs chan<- error) {
	eventsRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		err := s.ws.InitialData.RecentMessages.Enqueue(msg)
		if err != nil {
			s.logger.Errorf("failed to save message to InitialData: %v", err)
		}

		s.ws.Broadcasters.Messages.Broadcast(msg)

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

		return nil
	}

	messageRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		if msg.Layer == chat.SystemLayer && msg.
			Receiver == s.gs.Name {
			s.gs.Channel.ToServer <- msg
			return nil
		}

		err := s.gs.Broadcast(&msg)
		if err != nil {
			return errors.Wrap(err, "failed to broadcast message to clients")
		}

		return nil
	}

	procedureRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		if msg.Sender != s.gs.Name {
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
			return errors.Wrap(err, "failed to save message to memory")
		}

		return nil
	}

	routeMessages := chat.BuildRouter(
		s.gs.Channel.ToClients,
		printConsoleData,
		saveMessage,
		messageRoute,
		procedureRoute,
		eventsRoute,
	)

	go routeMessages(errs)
	go s.gs.ListenAndServe("tcp", "50051", errs)
}

func (s *ConlangServer) WebEvents(errs chan<- error) {
	go network.BroadcastTime(s.ws.Broadcasters.CurrentTime)
	go s.ws.ListenAndServe("7070", errs)
}

func (s *ConlangServer) StartProcedures(errs chan<- error) {
	go s.Evolve(errs)

	if s.config.debugEnabled {
		go func(errs chan<- error) {
			msgs, err := loadJsonFile[memory.Message]("./outputs/chats/chat_5.json")
			if err != nil {
				errs <- errors.Wrap(err, "failed to load test chats json file")
				return
			}

			gens, err := loadJsonFile[memory.Generation](
				"./outputs/generations/generations_20250502205519.json",
			)
			if err != nil {
				errs <- errors.Wrap(
					err,
					"failed to load test generation json file",
				)
				return
			}

			for _, v := range gens {
				err = s.ws.InitialData.RecentGenerations.Enqueue(v)
				if err != nil {
					errs <- errors.Wrap(
						err,
						"failed to enqueue recent generations",
					)
					return
				}
			}

			// s.logger.Debug(gens)

			// Output test data to channel.
			go s.outputTestData(msgs, gens)
		}(errs)
	}
}

func loadJsonFile[T any](p string) ([]T, error) {
	var a []T

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	b, _ := io.ReadAll(f)

	err = json.Unmarshal(b, &a)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (s *ConlangServer) Run(errs chan error) {
	s.Router(errs)
	s.WebEvents(errs)
	s.StartProcedures(errs)
}
