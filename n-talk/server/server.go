package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

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
	specification chat.LayerMessageSet
}

func NewConlangServer(
	name string,
	l *log.Logger,
	m LocalMemory,
	s chat.LayerMessageSet,
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
		err := s.sendCommand(commands.ClearMemory, v)
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
//func (s *ConlangServer) EvolutionLoop() {
//	specs := []string{
//		s.specification.Phonetics,
//		s.specification.Grammar,
//		s.specification.Dictionary,
//		s.specification.Logography,
//	}
//
//	for !s.systemAgentOnline {
//		// ...
//	}
//
//	for range 1 {
//		// Starts on Layer 4, recurses to 1.
//		specs = s.iterate(specs, 4)
//		// Save specs to memory
//		// send results to SYSTEM LLM
//		// Save result to LLM.
//	}
//}

func (s *ConlangServer) TestExchangeEvent(errs chan<- error) {
	// Wait for layer 1 to have required clients.

	allJoined := make(chan struct{})

	go func(c chan<- struct{}) {
		joined := false
		for !joined {
			time.Sleep(1 * time.Second)
			s.mu.Lock()
			v := s.clients.byLayerMap[chat.PhoneticsLayer]
			if len(v) >= 2 {
				joined = true
			}
			s.mu.Unlock()
		}
		close(c)
	}(allJoined)

	<-allJoined

	s.logger.Infof("%s clients all joined", chat.PhoneticsLayer)

	// Send the initialize command to first client.

	clients := s.getClientsByLayer(chat.PhoneticsLayer)

	s.logger.Infof("Sending %s to %s", commands.Unlatch, chat.PhoneticsLayer)
	for _, v := range clients {

		// First append new instructions to clients.

		content := s.specification[chat.PhoneticsLayer]

		err := s.sendCommand(commands.AppendInstructions, v, content)

		// Then unlatch them... They're ready.

		err = s.sendCommand(commands.Unlatch, v)
		if err != nil {
			s.logger.Error(err)
		}
	}

	var initMsg chat.Content = "Hello, let's begin."

	// Select the first client in the layer to be the initializer.

	initializerClient := s.getClientsByLayer(chat.PhoneticsLayer)[0]

	err := s.sendCommand(
		commands.SendInitialMessage,
		initializerClient,
		initMsg,
	)
	if err != nil {
		errs <- err
		return
	}

	i := 0

	for i < 7 {
		<-s.exchangeComplete
		i++
		s.logger.Infof("Exchange Total: %d", i)
	}

	for _, v := range clients {
		s.logger.Infof("Sending latch command to %s", v.name)
		err := s.sendCommand(commands.Latch, v)
		if err != nil {
			s.logger.Error(err)
		}

		s.logger.Infof("Sending clear memory command to %s", v.name)
		err = s.sendCommand(commands.ClearMemory, v)
		if err != nil {
			s.logger.Error(err)
		}
	}

	clients = s.getClientsByLayer(chat.GrammarLayer)

	i = 0

	for i < 7 {
		<-s.exchangeComplete
		i++
		s.logger.Infof("Exchange Total: %d", i)
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
