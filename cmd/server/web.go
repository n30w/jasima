package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"codeberg.org/n30w/jasima/memory"
	"github.com/charmbracelet/log"
)

type WebClient[T any] struct {
	send   chan T
	conn   http.ResponseWriter
	done   <-chan struct{}
	cancel context.CancelFunc
}

func (c *WebClient[T]) write() {
	rc := http.NewResponseController(c.conn)

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}

			j, err := json.Marshal(msg)
			if err != nil {
				c.cancel()
				return
			}

			d := string(j)

			_, err = fmt.Fprintf(c.conn, "data: %s\n\n", d)
			if err != nil {
				c.cancel()
				return
			}

			rc.Flush()

		case <-c.done:
			return
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

	b.logger.Infof("Web client connected %s", r.RemoteAddr)

	defer func() {
		b.mu.Lock()
		delete(b.webClients, client)
		b.mu.Unlock()
		client.cancel()
		close(client.send)
		b.logger.Infof("Web client disconnected %s", r.RemoteAddr)
	}()

	client.write()
}

type Broadcasters struct {
	messages        *Broadcaster[memory.Message]
	generation      *Broadcaster[generation]
	currentTime     *Broadcaster[string]
	testMessageFeed *Broadcaster[memory.Message]
}

func (s *ConlangServer) ListenAndServeWebEvents(
	port string,
	errs chan<- error,
) {
	p := makePortString(port)

	handler := http.NewServeMux()

	handler.HandleFunc("/time", s.broadcasters.currentTime.HandleClient)
	handler.HandleFunc("/chat", s.broadcasters.messages.HandleClient)
	handler.HandleFunc("/generations", s.broadcasters.generation.HandleClient)
	handler.HandleFunc("/test/chat", s.broadcasters.testMessageFeed.HandleClient)

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
