package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"

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
	name            chat.Name
	logger          *log.Logger
	memory          MemoryService
	gs              *network.ChatServer
	config          *config
	procedureChan   chan memory.Message
	dictUpdatesChan chan memory.ResponseDictionaryEntries
	procedures      chan utils.Queue[job]
	dictionary      memory.DictionaryGeneration
	generations     utils.Queue[memory.Generation]
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
		dictionaryGen1[k] = v
	}

	initialGen := memory.Generation{
		Transcript:     transcriptGen1,
		Logography:     logographyGen1,
		Specifications: specificationsGen1,
		Dictionary:     dictionaryGen1,
	}

	generations, err := utils.NewStaticFixedQueue[memory.Generation](
		cfg.procedures.maxGenerations + 1,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create generation queue")
	}

	err = generations.Enqueue(initialGen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to enqueue initial generation")
	}

	webServer, err := network.NewWebServer(l, errs, network.WithPort("7070"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create web server")
	}

	grpcServer, err := network.NewChatServer(l, errs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create grpc server")
	}

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
		name:          chat.Name(cfg.name),
		memory:        m,
		gs:            grpcServer,
		ws:            webServer,
		generations:   generations,
		procedureChan: make(chan memory.Message),
		// Make channel buffered with 1 spot, since it will only be used by that
		// many concurrent processes at a time.
		dictUpdatesChan: make(chan memory.ResponseDictionaryEntries, 1),
		procedures:      make(chan utils.Queue[job], 100),
		dictionary:      dictionaryGen1,
		config:          cfg,
		logger:          l,
		cmd:             network.BuildCommand(cfg.name),
		errs:            errs,
	}

	cs.sendCommands = network.SendCommandBuilder(cs.gs.Channel.ToClients)

	return cs, nil
}

func (s *ConlangServer) Router(ctx context.Context) {
	var (
		eventsRoute = func(ctx context.Context, pbMsg *chat.Message) error {
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

		printConsoleData = func(ctx context.Context, pbMsg *chat.Message) error {
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

		messageRoute = func(ctx context.Context, pbMsg *chat.Message) error {
			msg := *memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer, pbMsg.Command,
			)

			if msg.Layer == chat.SystemLayer && msg.
				Receiver == s.name {
				s.gs.Channel.ToServer <- msg
				return nil
			}

			err := s.gs.Broadcast(&msg)
			if err != nil {
				return errors.Wrap(err, "failed to broadcast message to clients")
			}

			return nil
		}

		procedureRoute = func(ctx context.Context, pbMsg *chat.Message) error {
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

		saveMessage = func(ctx context.Context, pbMsg *chat.Message) error {
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
	)

	routeMessages := chat.BuildRouter[*chat.Message](
		s.gs.Channel.ToClients,
		printConsoleData,
		saveMessage,
		messageRoute,
		procedureRoute,
		eventsRoute,
	)

	routeCtx, routeCancel := context.WithCancel(ctx)

	defer routeCancel()

	select {
	case <-routeCtx.Done():
		s.logger.Warn("Routing context cancelled")
	default:
		routeMessages(s.errs)
	}
}

func (s *ConlangServer) WebEvents(ctx context.Context) {
	var (
		timeNow = func(mux *http.ServeMux) {
			go network.BroadcastTime(s.ws.Broadcasters.CurrentTime)

			mux.HandleFunc("/time", s.ws.Broadcasters.CurrentTime.HandleClient)
		}

		chatting = func(mux *http.ServeMux) {
			mux.HandleFunc(
				"/chat", s.ws.Broadcasters.Messages.InitialData(
					s.ws.InitialData.RecentMessages,
				),
			)
			mux.HandleFunc(
				"/wordDetection",
				s.ws.Broadcasters.MessageWordDictExtraction.InitialData(
					s.ws.InitialData.RecentUsedWords,
				),
			)
		}

		generations = func(mux *http.ServeMux) {
			mux.HandleFunc(
				"/specifications",
				s.ws.Broadcasters.Specification.InitialData(s.ws.InitialData.RecentSpecifications),
			)
			mux.HandleFunc(
				"/generations",
				s.ws.Broadcasters.Generation.InitialData(s.ws.InitialData.RecentGenerations),
			)
		}

		logograms = func(mux *http.ServeMux) {
			mux.HandleFunc(
				"/logograms/display",
				s.ws.Broadcasters.LogogramDisplay.InitialData(s.ws.InitialData.RecentLogogram),
			)
		}

		testing = func(mux *http.ServeMux) {
			mux.HandleFunc(
				"/test/chat",
				s.ws.Broadcasters.TestMessageFeed.InitialData(s.ws.InitialData.RecentMessages),
			)
			mux.HandleFunc(
				"/test/generations",
				s.ws.Broadcasters.TestGenerationsFeed.InitialData(
					s.ws.InitialData.RecentGenerations,
				),
			)
		}
	)

	s.ws.ListenAndServe(
		ctx,
		timeNow,
		chatting,
		generations,
		logograms,
		testing,
	)
}

func (s *ConlangServer) ProcessJobs(ctx context.Context) {
	jobsCtx := context.WithoutCancel(ctx)

	for {
		select {
		case <-jobsCtx.Done():
			s.logger.Warn("Cancelling jobs")
			return
		case jobs := <-s.procedures:
			for j, err := jobs.Dequeue(); err == nil; j, err = jobs.Dequeue() {
				err = j(jobsCtx)
				if err != nil {
					s.errs <- err
					return
				}
			}
		}
	}
}

func (s *ConlangServer) Run(ctx context.Context, wg *sync.WaitGroup) {
	jobs, _ := utils.NewStaticFixedQueue[job](100)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.gs.ListenAndServe(ctx)
		if err != nil {
			s.errs <- err
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		go s.Router(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.WebEvents(ctx)
	}()

	// wg.Add(1)
	go func() {
		// defer wg.Done()

		_ = jobs.Enqueue(
			s.WaitForClients(11),
			s.Evolve,
		)

		s.procedures <- jobs

		s.ProcessJobs(ctx)
	}()

	if s.config.debugEnabled && s.config.broadcastTestData {
		wg.Add(1)
		go func() {
			defer wg.Done()

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

			// Output test data to channel.
			s.outputTestData(ctx, msgs, gens)
		}()
	}
}
