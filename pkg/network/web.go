package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type WebServer struct {
	InitialData  *InitialData
	Broadcasters *Broadcasters
	logger       *log.Logger
}

func NewWebServer(l *log.Logger) (*WebServer, error) {
	b := NewBroadcasters(l)
	i, err := NewInitialData()
	if err != nil {
		return nil, err
	}
	return &WebServer{
		InitialData:  i,
		Broadcasters: b,
		logger:       l,
	}, nil
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

		c := &WebClient[T]{
			send:   make(chan T, 10),
			conn:   w,
			done:   ctx.Done(),
			cancel: cancel,
		}

		b.mu.Lock()
		b.webClients[c] = struct{}{}
		b.mu.Unlock()

		b.logger.Infof("Web Client connected @ %s", r.RemoteAddr)

		defer func() {
			b.mu.Lock()
			delete(b.webClients, c)
			b.mu.Unlock()
			c.cancel()
			close(c.send)
			b.logger.Infof("Web Client disconnected @ %s", r.RemoteAddr)
		}()

		// Send initial data so there's something to see on the frontend, other than
		// just blank data. Make a copy of the queue that is passed in so that we do
		// not actually discard the stuff inside it. After all, we're using
		// a pointer to reference its current value for each new GRPCClient.

		q, err := d.ToSlice()
		if err != nil {
			b.logger.Error(err)
		}

		for i := range len(q) {
			c.send <- q[i]
		}

		// Then serve. This function loops until the GRPCClient disconnects.

		err = c.serve()
		if err != nil {
			c.cancel()
		}
	}
}

func (b *Broadcaster[T]) HandleClient(w http.ResponseWriter, r *http.Request) {
	addEventHeaders(w)

	ctx, cancel := context.WithCancel(r.Context())

	c := &WebClient[T]{
		send:   make(chan T, 10),
		conn:   w,
		done:   ctx.Done(),
		cancel: cancel,
	}

	b.mu.Lock()
	b.webClients[c] = struct{}{}
	b.mu.Unlock()

	b.logger.Infof("Web GRPCClient connected @ %s", r.RemoteAddr)

	defer func() {
		b.mu.Lock()
		delete(b.webClients, c)
		b.mu.Unlock()
		c.cancel()
		close(c.send)
		b.logger.Infof("Web GRPCClient disconnected @ %s", r.RemoteAddr)
	}()

	err := c.serve()
	if err != nil {
		c.cancel()
	}
}

type Broadcasters struct {
	Messages            *Broadcaster[memory.Message]
	Generation          *Broadcaster[memory.Generation]
	Specification       *Broadcaster[memory.SpecificationGeneration]
	CurrentTime         *Broadcaster[string]
	TestMessageFeed     *Broadcaster[memory.Message]
	TestGenerationsFeed *Broadcaster[memory.Generation]
}

func NewBroadcasters(l *log.Logger) *Broadcasters {
	return &Broadcasters{
		Messages:            NewBroadcaster[memory.Message](l),
		Generation:          NewBroadcaster[memory.Generation](l),
		Specification:       NewBroadcaster[memory.SpecificationGeneration](l),
		CurrentTime:         NewBroadcaster[string](l),
		TestMessageFeed:     NewBroadcaster[memory.Message](l),
		TestGenerationsFeed: NewBroadcaster[memory.Generation](l),
	}
}

func (s WebServer) ListenAndServe(
	port string,
	errs chan<- error,
) {
	p := makePortString(port)

	handler := http.NewServeMux()

	handler.HandleFunc("/time", s.Broadcasters.CurrentTime.HandleClient)
	handler.HandleFunc(
		"/specifications",
		s.Broadcasters.Specification.InitialData(s.InitialData.RecentSpecifications),
	)
	handler.HandleFunc(
		"/chat", s.Broadcasters.Messages.InitialData(
			s.
				InitialData.RecentMessages,
		),
	)
	handler.HandleFunc(
		"/generations",
		s.Broadcasters.Generation.InitialData(s.InitialData.RecentGenerations),
	)
	handler.HandleFunc(
		"/test/chat",
		s.Broadcasters.TestMessageFeed.InitialData(s.InitialData.RecentMessages),
	)
	handler.HandleFunc(
		"/test/generations",
		s.Broadcasters.TestGenerationsFeed.InitialData(s.InitialData.RecentGenerations),
	)

	s.logger.Infof("Starting web events service on %s", p)

	err := http.ListenAndServe(p, handler)
	if err != nil {
		errs <- err
		return
	}
}

func BroadcastTime(b *Broadcaster[string]) {
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

// InitialData contains frontend initializing data so that, when connected,
// data is shown rather than having nothing.
type InitialData struct {
	RecentMessages       *utils.FixedQueue[memory.Message]
	RecentGenerations    *utils.FixedQueue[memory.Generation]
	RecentSpecifications *utils.FixedQueue[memory.SpecificationGeneration]
}

func NewInitialData() (*InitialData, error) {
	recentMessagesQueue, err := utils.NewFixedQueue[memory.Message](10)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate recent messages queue")
	}

	rg, err := utils.NewFixedQueue[memory.Generation](100)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"failed to make recent generations queue",
		)
	}

	specs, err := utils.NewFixedQueue[memory.SpecificationGeneration](10)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"failed to make recent specifications queue",
		)
	}

	initData := &InitialData{
		RecentMessages:       recentMessagesQueue,
		RecentGenerations:    rg,
		RecentSpecifications: specs,
	}
	return initData, nil
}

type WebClient[T any] struct {
	send   chan T
	conn   http.ResponseWriter
	done   <-chan struct{}
	cancel context.CancelFunc
}

func (c *WebClient[T]) serve() error {
	errMsg := "failed serving GRPCClient"

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
