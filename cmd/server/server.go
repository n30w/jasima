package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

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

type Job func(ctx context.Context) error

type ConlangServer struct {
	name            chat.Name
	logger          *log.Logger
	memory          MemoryService
	gs              *network.ChatServer
	config          *config
	procedureChan   chan memory.Message
	dictUpdatesChan chan memory.ResponseDictionaryEntries
	procedures      chan utils.Queue[jobs]
	dictionary      memory.DictionaryGeneration
	generations     utils.Queue[memory.Generation]
	ws              *network.WebServer
	errs            chan error

	// cmd builds commands that can be sent to an agent.
	cmd network.CommandForAgent
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
		procedureChan: make(chan memory.Message, 100),
		// Make channel buffered with 1 spot, since it will only be used by that
		// many concurrent processes at a time.
		dictUpdatesChan: make(chan memory.ResponseDictionaryEntries, 1),
		procedures:      make(chan utils.Queue[[]job], 100),
		dictionary:      dictionaryGen1,
		config:          cfg,
		logger:          l,
		cmd:             network.BuildCommand(cfg.name),
		errs:            errs,
	}

	return cs, nil
}

func (s *ConlangServer) Router(ctx context.Context) {
	var (
		msg memory.Message

		eventsRoute = func(ctx context.Context, pbMsg *chat.Message) error {
			err := s.ws.InitialData.RecentMessages.Enqueue(msg)
			if err != nil {
				s.logger.Errorf("failed to save message to InitialData: %v", err)
			}

			return nil
		}

		printConsoleData = func(ctx context.Context, pbMsg *chat.Message) error {
			// s.logger.Debugf("MESSAGE: %+v", msg)

			if msg.Command != agent.NoCommand {
				s.logger.Debugf(
					"Issued command %s to %s", msg.Command,
					msg.Receiver,
				)
			}

			return nil
		}

		messageRoute = func(ctx context.Context, pbMsg *chat.Message) error {
			if msg.Layer == chat.SystemLayer && msg.Receiver == s.name {
				return nil
			}

			return s.gs.Broadcast(&msg)
		}

		procedureRoute = func(ctx context.Context, pbMsg *chat.Message) error {
			isAgentMsg := msg.Sender != s.name && msg.Command == agent.NoCommand

			// Match any message solely from an agent.
			if isAgentMsg && msg.Layer != chat.SystemLayer {
				return utils.SendWithContext(
					ctx,
					s.procedureChan,
					msg,
					func() {
						s.logger.Debug(
							"Dispatching message to procedure channel",
							"sender",
							msg.Sender,
							"receiver",
							msg.Receiver,
							"layer",
							msg.Layer,
						)
					},
				)
			} else if isAgentMsg && msg.Layer == chat.SystemLayer {
				return utils.SendWithContext(
					ctx,
					s.gs.Channel.ToServer,
					msg,
					func() {
						s.logger.Debug("msg sent to system channel")
					},
				)
			}

			return nil
		}

		saveMessage = func(ctx context.Context, pbMsg *chat.Message) error {
			return saveMessageTo(ctx, s.memory, msg)
		}
	)

	routeMessages := chat.BuildRouter[*chat.Message](
		s.gs.Channel.ToClients,
		// Clever for no reason. Don't do this.
		func(ctx context.Context, pbMsg *chat.Message) error {
			msg = *memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer, pbMsg.Command,
			)
			return nil
		},
		printConsoleData,
		saveMessage,
		messageRoute,
		procedureRoute,
		eventsRoute,
	)

	err := routeMessages(ctx)
	if err != nil {
		s.logger.Errorf("failed to route messages: %v", err)
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

	webCtx, webCancel := context.WithCancel(ctx)
	defer webCancel()

	s.ws.ListenAndServe(
		webCtx,
		timeNow,
		chatting,
		generations,
		logograms,
		testing,
	)
}

func (s *ConlangServer) ProcessJobs(ctx context.Context) {
	for procs := range s.procedures {
		for p, err := procs.Dequeue(); err == nil; p, err = procs.Dequeue() {
			select {
			case <-ctx.Done():
				s.logger.Warn("Processing context cancelled")
				return
			default:
				for _, j := range p {
					select {
					case <-ctx.Done():
						s.logger.Warn("Processing context cancelled")
						return
					default:
						err = j.do(ctx)
						if err != nil {
							s.errs <- err
							return
						}
						s.logger.Infof("job complete: %s", j)
					}
				}
			}
		}
	}
}

func (s *ConlangServer) Run(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.gs.ListenAndServe(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.WebEvents(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Router(ctx)
	}()

	// wg.Add(1)
	go func() {
		// defer wg.Done()
		t := utils.Timer(time.Now())

		var (
			err error

			// ng is the new generation that will be used throughout batches
			// of jobs to evolve the language.
			ng = &memory.Generation{}

			// i keeps track of the current generation.
			i = 0

			waitForClientsProc = &procedure{
				name: "wait-for-clients",
				exec: s.WaitForClients(11),
			}

			iterateSpecsProc = &procedure{
				name: "iterate-specifications",
				exec: s.iterateSpecs(i, ng),
			}

			iterateDictionaryProc = &procedure{
				name: "iterate-dictionary",
				exec: s.iterateDictionary(i, ng),
			}

			iterateLogogramsProc = &procedure{
				name: "iterate-logograms",
				exec: s.iterateLogograms(i, ng),
			}

			updateGenerationsProc = &procedure{
				name: "update-generations",
				exec: s.updateGenerations(i, ng),
			}

			waitProcedureProc = &procedure{
				name: "wait-procedure",
				exec: s.wait(10 * time.Second),
			}

			exportDataProc = &procedure{
				name: "export-data",
				exec: s.exportData(t),
			}

			initializeProcs = jobs{
				waitForClientsProc,
			}

			evolveProcs = jobs{
				iterateSpecsProc,
				iterateDictionaryProc,
				iterateLogogramsProc,
				updateGenerationsProc,
				waitProcedureProc,
			}

			exportProcs = jobs{
				exportDataProc,
			}
		)

		q, _ := utils.NewStaticFixedQueue[jobs](1)

		_ = q.Enqueue(initializeProcs)

		err = utils.SendWithContext(ctx, s.procedures, q)
		if err != nil {
			s.errs <- err
			return
		}

		// Add the configured number of generation iterations to the queue.

		gq, _ := utils.NewStaticFixedQueue[jobs](s.config.procedures.maxGenerations)

		for range s.config.procedures.maxGenerations {
			_ = gq.Enqueue(evolveProcs)

			err = utils.SendWithContext(ctx, s.procedures, gq)
			if err != nil {
				s.errs <- err
				return
			}
		}

		eq, _ := utils.NewStaticFixedQueue[jobs](1)

		_ = eq.Enqueue(exportProcs)

		err = utils.SendWithContext(ctx, s.procedures, eq)
		if err != nil {
			s.errs <- err
			return
		}

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

func (s *ConlangServer) Teardown() {
	close(s.procedureChan)
	close(s.dictUpdatesChan)
	close(s.procedures)
}
