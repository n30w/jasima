package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"codeberg.org/n30w/jasima/memory"
	"codeberg.org/n30w/jasima/utils"
	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type WebClient[T any] struct {
	send   chan T
	conn   http.ResponseWriter
	done   <-chan struct{}
	cancel context.CancelFunc
}

func (c *WebClient[T]) serve() error {
	errMsg := "failed serving client"

	rc := http.NewResponseController(c.conn)

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return nil
			}

			j, err := json.Marshal(msg)
			if err != nil {
				return errors.Wrap(err, errMsg)
			}

			d := string(j)

			_, err = fmt.Fprintf(c.conn, "data: %s\n\n", d)
			if err != nil {
				return errors.Wrap(err, errMsg)
			}

			rc.Flush()

		case <-c.done:
			return nil
		}
	}
}

type Broadcaster[T any] struct {
	mu         sync.Mutex
	webClients map[*WebClient[T]]struct{}
	logger     *log.Logger
}

func NewBroadcaster[T any](logger *log.Logger) *Broadcaster[T] {
	return &Broadcaster[T]{
		mu:         sync.Mutex{},
		webClients: make(map[*WebClient[T]]struct{}),
		logger:     logger,
	}
}

func (b *Broadcaster[T]) Broadcast(msg T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for c := range b.webClients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (b *Broadcaster[T]) InitialData(d *utils.FixedQueue[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addEventHeaders(w)

		ctx, cancel := context.WithCancel(r.Context())

		client := &WebClient[T]{
			send:   make(chan T, 10),
			conn:   w,
			done:   ctx.Done(),
			cancel: cancel,
		}

		b.mu.Lock()
		b.webClients[client] = struct{}{}
		b.mu.Unlock()

		b.logger.Infof("Web client connected @ %s", r.RemoteAddr)

		defer func() {
			b.mu.Lock()
			delete(b.webClients, client)
			b.mu.Unlock()
			client.cancel()
			close(client.send)
			b.logger.Infof("Web client disconnected @ %s", r.RemoteAddr)
		}()

		// Send initial data so there's something to see on the frontend, other than
		// just blank data. Make a copy of the queue that is passed in so that we do
		// not actually discard the stuff inside it. After all, we're using
		// a pointer to reference its current value for each new client.

		q, err := d.ToSlice()
		if err != nil {
			b.logger.Error(err)
		}

		for i := range len(q) {
			client.send <- q[i]
		}

		// Then serve. This function loops until the client disconnects.

		err = client.serve()
		if err != nil {
			client.cancel()
		}
	}
}

func (b *Broadcaster[T]) HandleClient(w http.ResponseWriter, r *http.Request) {
	addEventHeaders(w)

	ctx, cancel := context.WithCancel(r.Context())

	client := &WebClient[T]{
		send:   make(chan T, 10),
		conn:   w,
		done:   ctx.Done(),
		cancel: cancel,
	}

	b.mu.Lock()
	b.webClients[client] = struct{}{}
	b.mu.Unlock()

	b.logger.Infof("Web client connected @ %s", r.RemoteAddr)

	defer func() {
		b.mu.Lock()
		delete(b.webClients, client)
		b.mu.Unlock()
		client.cancel()
		close(client.send)
		b.logger.Infof("Web client disconnected @ %s", r.RemoteAddr)
	}()

	client.serve()
}

type Broadcasters struct {
	messages            *Broadcaster[memory.Message]
	generation          *Broadcaster[memory.Generation]
	specification       *Broadcaster[memory.SpecificationGeneration]
	currentTime         *Broadcaster[string]
	testMessageFeed     *Broadcaster[memory.Message]
	testGenerationsFeed *Broadcaster[memory.Generation]
}

func NewBroadcasters(l *log.Logger) *Broadcasters {
	return &Broadcasters{
		messages:            NewBroadcaster[memory.Message](l),
		generation:          NewBroadcaster[memory.Generation](l),
		specification:       NewBroadcaster[memory.SpecificationGeneration](l),
		currentTime:         NewBroadcaster[string](l),
		testMessageFeed:     NewBroadcaster[memory.Message](l),
		testGenerationsFeed: NewBroadcaster[memory.Generation](l),
	}
}

func (s *ConlangServer) ListenAndServeWebEvents(
	port string,
	errs chan<- error,
) {
	p := makePortString(port)

	handler := http.NewServeMux()

	handler.HandleFunc("/time", s.broadcasters.currentTime.HandleClient)
	handler.HandleFunc(
		"/specifications",
		s.broadcasters.specification.InitialData(s.initialData.recentSpecifications),
	)
	handler.HandleFunc(
		"/chat", s.broadcasters.messages.InitialData(
			s.
				initialData.recentMessages,
		),
	)
	handler.HandleFunc(
		"/generations",
		s.broadcasters.generation.InitialData(s.generations),
	)
	handler.HandleFunc(
		"/test/chat",
		s.broadcasters.testMessageFeed.InitialData(s.initialData.recentMessages),
	)
	handler.HandleFunc(
		"/test/generations",
		s.broadcasters.testGenerationsFeed.InitialData(s.initialData.recentGenerations),
	)

	s.logger.Infof("Starting web events service on %s", p)

	err := http.ListenAndServe(p, handler)
	if err != nil {
		errs <- err
		return
	}
}

func broadcastTime(b *Broadcaster[string]) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		<-t.C
		d := time.Now().Format(time.UnixDate)
		b.Broadcast(d)
	}
}

func addEventHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
