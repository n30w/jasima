package main

import (
	"context"
	"fmt"

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

type job func(ctx context.Context) error

type ConlangServer struct {
	logger          *log.Logger
	memory          MemoryService
	gs              *network.GRPCServer
	config          *config
	procedureChan   chan memory.Message
	dictUpdatesChan chan chat.DictionaryEntriesResponse
	procedures      chan *utils.FixedQueue[job]
	dictionary      memory.DictionaryGeneration
	generations     *utils.FixedQueue[memory.Generation]
	ws              *network.WebServer
	errs            chan error

	// cmd builds commands that can be sent to an agent.
	cmd network.CommandForAgent

	// sendCommands sends commands to agents.
	sendCommands network.CommandsSender
}

func NewConlangServer(
	cfg *config,
	l *log.Logger,
	m MemoryService,
	errs chan error,
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

	// Then for each entry of the dictionary, go ahead and set the logogram
	// values to the SVGs that had been loaded.

	for k := range dictionaryGen1 {
		v := dictionaryGen1[k]
		v.Logogram = logographyGen1[k]
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

	err = webServer.InitialData.RecentGenerations.Enqueue(initialGen)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"failed to enqueue initial generation to recent generations",
		)
	}

	cs := &ConlangServer{
		memory:        m,
		gs:            grpcServer,
		ws:            webServer,
		generations:   generations,
		procedureChan: make(chan memory.Message),
		// Make channel buffered with 1 spot, since it will only be used by that
		// many concurrent processes at a time.
		dictUpdatesChan: make(chan chat.DictionaryEntriesResponse, 1),
		procedures:      make(chan *utils.FixedQueue[job], 100),
		dictionary:      dictionaryGen1,
		config:          cfg,
		logger:          l,
		cmd:             network.BuildCommand(cfg.name),
		errs:            errs,
	}

	cs.sendCommands = network.SendCommandBuilder(cs.gs.Channel.ToClients)

	return cs, nil
}

func (s *ConlangServer) Router() {
	eventsRoute := func(ctx context.Context, pbMsg *chat.Message) error {
		msg := *memory.NewChatMessage(
			pbMsg.Sender, pbMsg.Receiver,
			pbMsg.Content, pbMsg.Layer, pbMsg.Command,
		)

		err := s.ws.InitialData.RecentMessages.Enqueue(msg)
		if err != nil {
			s.logger.Errorf("failed to save message to InitialData: %v", err)
		}

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

	go routeMessages(s.errs)
	go s.gs.ListenAndServe("tcp", "50051", s.errs)
}

func (s *ConlangServer) WebEvents() {
	go network.BroadcastTime(s.ws.Broadcasters.CurrentTime)
	go s.ws.ListenAndServe("7070", s.errs)
}

func (s *ConlangServer) StartProcedures() {
	jobs, _ := utils.NewFixedQueue[job](100)

	_ = jobs.Enqueue(s.WaitForClients(11))
	_ = jobs.Enqueue(s.Evolve)

	s.procedures <- jobs

	if s.config.debugEnabled && s.config.broadcastTestData {
		go func() {
			msgs, err := loadJsonFile[memory.Message]("./outputs/chats/chat_5.json")
			if err != nil {
				s.errs <- errors.Wrap(
					err,
					"failed to load test chats json file",
				)
				return
			}

			gens, err := loadJsonFile[memory.Generation](
				"./outputs/generations/generations_20250502205519.json",
			)
			if err != nil {
				s.errs <- errors.Wrap(
					err,
					"failed to load test generation json file",
				)
				return
			}

			for _, v := range gens {
				err = s.ws.InitialData.RecentGenerations.Enqueue(v)
				if err != nil {
					s.errs <- errors.Wrap(
						err,
						"failed to enqueue recent generations",
					)
					return
				}
			}

			// s.logger.Debug(gens)

			// Output test data to channel.
			go s.outputTestData(msgs, gens)
		}()
	}
}

func (s *ConlangServer) ProcessJobs() {
	for jobs := range s.procedures {
		for j, err := jobs.Dequeue(); err == nil; j, err = jobs.Dequeue() {
			ctx, cancel := context.WithCancel(context.Background())
			err = j(ctx)
			if err != nil {
				s.errs <- err
				cancel()
				return
			}
			cancel()
		}
	}
}

func (s *ConlangServer) Run() {
	s.Router()
	s.WebEvents()
	s.StartProcedures()
	go s.ProcessJobs()
}
