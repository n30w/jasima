package server

import (
	"context"
	"os"
	"path/filepath"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/commands"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"

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
}

type ConlangServer struct {
	Server

	// systemAgentOnline signifies whether the system LLM agent service has
	// come online or not. The evolution cannot start without it.
	systemAgentOnline bool

	// specification are serialized versions of the Markdown specifications.
	specification *LangSpecification
}

func NewConlangServer(
	name string,
	l *log.Logger,
	m LocalMemory,
	s *LangSpecification,
) *ConlangServer {
	return &ConlangServer{
		Server: Server{
			clients: &clientele{
				byNameMap:  make(nameToClientsMap),
				byLayerMap: make(layerToNamesMap),
				logger:     l,
			},
			name:             chat.Name(name),
			logger:           l,
			memory:           m,
			exchangeComplete: make(chan bool),
		},
		specification: s,
	}
}

// iterate begins the processing of a Layer. The function completes after the
// total number of back and forth rounds are complete. Layer control and message
// routing are decoupled.
func (s *ConlangServer) iterate(specs []string, layer int32) []string {
	newSpecs := make([]string, 0)

	if layer == 0 {
		return newSpecs
	}

	// Compile previous Layer's outputs to use in this current Layer's input

	newSpecs = append(newSpecs, s.iterate(specs[:layer], layer-1)...)

	currentLayer := chat.Layer(layer)

	clients := s.getClientsByLayer(currentLayer)

	// Dispatch iterate commands to clients on Layer.

	// Send prevSpec to clients. Compile specs into a single system instruction
	// for LLM.

	// Unlatch clients.

	exchanges := 20

	for range exchanges {
		<-s.exchangeComplete
	}

	// latch clients

	// Send every client in the Layer clear memory command.
	for _, v := range clients {
		err := s.SendCommand(commands.ClearMemory, v)
		if err != nil {
		}
	}

	// When the chatting is complete, compile the chat records and
	// send to SYSTEM LLM service.
	//
	// For each Layer, ask for updates on specification.

	// When SYSTEM LLM sends response back, adjust the corresponding
	// specification.

	return newSpecs
}

// querySystemAgent makes a query to the system LLM agent. This query is often
// to compile and summarize the data generated during the discussion phase.
//func (s *ConlangServer) querySystemAgent(data string) {
//	// Check if the system agent is online.
//	// The response received should be deserialized into array data.
//	clients := s.getClientsByLayer(0)
//}

// EvolutionLoop manages the entire evolutionary function loop.
func (s *ConlangServer) EvolutionLoop() {
	specs := []string{
		s.specification.Phonetics,
		s.specification.Grammar,
		s.specification.Dictionary,
		s.specification.Logography,
	}

	for !s.systemAgentOnline {
		// ...
	}

	for range 1 {
		// Starts on Layer 4, recurses to 1.
		specs = s.iterate(specs, 4)
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
