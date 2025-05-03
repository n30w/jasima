package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"codeberg.org/n30w/jasima/agent"
	"codeberg.org/n30w/jasima/utils"

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

// initialData contains frontend initializing data so that, when connected,
// data is shown rather than having nothing.
type initialData struct {
	recentMessages    *utils.FixedQueue[memory.Message]
	recentGenerations *utils.FixedQueue[memory.Generation]
}

type ConlangServer struct {
	Server
	config        *config
	procedureChan chan memory.Message
	generations   *utils.FixedQueue[memory.Generation]
	broadcasters  *Broadcasters
	initialData   *initialData
}

type procedureConfig struct {
	// maxExchanges represents the total exchanges allowed per layer
	// of evolution.
	maxExchanges int

	// maxGenerations represents the maximum number of generations to evolve.
	// When set to 0, the procedure evolves forever.
	maxGenerations int
}

type filePathConfig struct {
	specifications string
	logography     string
	dictionary     string
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
		errMsg = "failed to initialize new conlang server"
		c      = channels{
			messagePool:            make(chan *chat.Message),
			systemLayerMessagePool: make(memory.MessageChannel),
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

	transcriptGen1 := newTranscriptGeneration()

	// Load and serialize specifications.

	specificationsGen1, err := loadSpecificationsFromFile(
		cfg.files.
			specifications,
	)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	// Load and serialize logography SVGs.

	logographyGen1, err := loadLogographySvgsFromFile(cfg.files.logography)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	// Load and serialize dictionary.

	dictionaryGen1, err := loadDictionaryFromFile(cfg.files.dictionary)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
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
		return nil, errors.Wrap(err, errMsg)
	}

	err = generations.Enqueue(initialGen)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	// Initialize web broadcasters

	b := &Broadcasters{
		messages:            NewBroadcaster[memory.Message](l),
		generation:          NewBroadcaster[memory.Generation](l),
		currentTime:         NewBroadcaster[string](l),
		testMessageFeed:     NewBroadcaster[memory.Message](l),
		testGenerationsFeed: NewBroadcaster[memory.Generation](l),
	}

	recentMessagesQueue, err := utils.NewFixedQueue[memory.Message](10)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	rg, err := utils.NewFixedQueue[memory.Generation](100)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"failed to make new generation initial data",
		)
	}

	initData := &initialData{
		recentMessages:    recentMessagesQueue,
		recentGenerations: rg,
	}

	return &ConlangServer{
		Server: Server{
			clients:   ct,
			name:      chat.Name(cfg.name),
			logger:    l,
			memory:    m,
			channels:  c,
			listening: true,
		},
		initialData:   initData,
		generations:   generations,
		procedureChan: make(chan memory.Message),
		broadcasters:  b,
		config:        cfg,
	}, nil
}

func (s *ConlangServer) Router(errs chan<- error) {
	errMsg := "failed to route message"

	eventsRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		err := s.initialData.recentMessages.Enqueue(msg)
		if err != nil {
			s.logger.Errorf("failed to save message to initialData: %v", err)
		}

		s.broadcasters.messages.Broadcast(msg)

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

		if msg.Layer == chat.SystemLayer && msg.
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

	routeMessages := chat.BuildRouter(
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
	go broadcastTime(s.broadcasters.currentTime)
	go s.ListenAndServeWebEvents("7070", errs)
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

			// gens, err := loadJsonFile[memory.Generation]("./outputs/generations/generations_20250430160258.json")
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
				if err := s.initialData.recentGenerations.Enqueue(v); err != nil {
					errs <- errors.Wrap(
						err,
						"failed to enqueue recent generations",
					)
					return
				}
			}

			// gens := []memory.Generation{}

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
