package server

import (
	"context"
	"os"
	"path/filepath"

	"codeberg.org/n30w/jasima/n-talk/memory"
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
	Retrieve(ctx context.Context, name string, n int) ([]memory.Message, error)

	Clear() error
}

type ServerMemoryService interface {
	MemoryService
	String() string
}

type ConlangServer struct {
	Server

	// specification are serialized versions of the Markdown specifications.
	specification *LangSpecification
}

func NewConlangServer(
	name string,
	l *log.Logger,
	m ServerMemoryService,
	s *LangSpecification,
) *ConlangServer {
	return &ConlangServer{
		Server: Server{
			clients: &clientele{
				byName:  make(nameToClientsMap),
				byLayer: make(layerToNamesMap),
				logger:  l,
			},
			serverName:       name,
			logger:           l,
			memory:           m,
			exchangeComplete: make(chan bool),
		},
		specification: s,
	}
}

// process begins the processing of a layer. The function completes after the
// total number of back and forth rounds are complete. Layer control and message
// routing are decoupled.
func (s *ConlangServer) process(specs []string, layer int32) []string {
	newSpecs := make([]string, 0)

	if layer == 0 {
		return newSpecs
	}

	// Compile previous layer's outputs to use in this current layer's input

	newSpecs = append(newSpecs, s.process(specs[:layer], layer-1)...)

	clients := s.getClientsByLayer(layer)

	// Dispatch process commands to clients on layer.

	// Send prevSpec to clients. Compile specs into a single system instruction
	// for LLM.

	// Unlatch clients.

	exchanges := 20

	for range exchanges {
		<-s.exchangeComplete
	}

	// latch clients

	// Send every client in the layer clear memory command.
	for _, v := range clients {
		err := s.SendCommand(ClearMemory, v)
		if err != nil {
		}
	}

	// When the chatting is complete, compile the chat records and
	// send to SYSTEM LLM service.
	//
	// For each layer, ask for updates on specification.

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.

	return newSpecs
}

// EvolutionLoop manages the entire evolutionary function loop.
func (s *ConlangServer) EvolutionLoop() {
	specs := []string{
		s.specification.Phonetics,
		s.specification.Grammar,
		s.specification.Dictionary,
		s.specification.Logography,
	}

	for range 1 {
		// Starts on layer 4, recurses to 1.
		specs = s.process(specs, 4)
		// Save specs to memory
		// send results to SYSTEM LLM
		// Save result to LLM.
	}
}

func (s *Server) TestExchangeEvent() {
	i := 0
	for i < 5 {
		<-s.exchangeComplete
		i++
		s.logger.Infof("Exchange Total: %d", i)
	}
	s.logger.Info("see ya later")
}

type LangSpecification struct {
	Logography string
	Grammar    string
	Dictionary string
	Phonetics  string
}

func NewLangSpecification(p string) (*LangSpecification, error) {
	ls := &LangSpecification{}

	b, err := os.ReadFile(filepath.Join(p, "dictionary.md"))
	if err != nil {
		return nil, err
	}

	ls.Dictionary = string(b)

	b, err = os.ReadFile(filepath.Join(p, "grammar.md"))
	if err != nil {
		return nil, err
	}

	ls.Grammar = string(b)

	b, err = os.ReadFile(filepath.Join(p, "logography.md"))
	if err != nil {
		return nil, err
	}

	ls.Logography = string(b)

	b, err = os.ReadFile(filepath.Join(p, "phonetics.md"))
	if err != nil {
		return nil, err
	}

	ls.Phonetics = string(b)

	return ls, nil
}
