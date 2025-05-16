package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type WebServer struct {
	InitialData  *InitialData
	Broadcasters *Broadcasters
	logger       *log.Logger
	errs         chan<- error
}

func NewWebServer(l *log.Logger, errs chan<- error) (*WebServer, error) {
	b := NewBroadcasters(l)
	i, err := NewInitialData()
	if err != nil {
		return nil, err
	}
	return &WebServer{
		InitialData:  i,
		Broadcasters: b,
		logger:       l,
		errs:         errs,
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
		// a pointer to reference its current value for each new ChatClient.

		q, err := d.ToSlice()
		if err != nil {
			b.logger.Error(err)
		}

		for i := range len(q) {
			c.send <- q[i]
		}

		// Then serve. This function loops until the ChatClient disconnects.

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

	b.logger.Infof("Web ChatClient connected @ %s", r.RemoteAddr)

	defer func() {
		b.mu.Lock()
		delete(b.webClients, c)
		b.mu.Unlock()
		c.cancel()
		close(c.send)
		b.logger.Infof("Web ChatClient disconnected @ %s", r.RemoteAddr)
	}()

	err := c.serve()
	if err != nil {
		c.cancel()
	}
}

type Broadcasters struct {
	Messages                  *Broadcaster[memory.Message]
	MessageWordDictExtraction *Broadcaster[chat.AgentDictionaryWordsDetectionResponse]
	Generation                *Broadcaster[memory.Generation]
	Specification             *Broadcaster[memory.SpecificationGeneration]
	LogogramDisplay           *Broadcaster[chat.LogogramIteration]
	CurrentTime               *Broadcaster[string]
	TestMessageFeed           *Broadcaster[memory.Message]
	TestGenerationsFeed       *Broadcaster[memory.Generation]
}

func NewBroadcasters(l *log.Logger) *Broadcasters {
	return &Broadcasters{
		Messages:                  NewBroadcaster[memory.Message](l),
		MessageWordDictExtraction: NewBroadcaster[chat.AgentDictionaryWordsDetectionResponse](l),
		Generation:                NewBroadcaster[memory.Generation](l),
		Specification:             NewBroadcaster[memory.SpecificationGeneration](l),
		LogogramDisplay:           NewBroadcaster[chat.LogogramIteration](l),
		CurrentTime:               NewBroadcaster[string](l),
		TestMessageFeed:           NewBroadcaster[memory.Message](l),
		TestGenerationsFeed:       NewBroadcaster[memory.Generation](l),
	}
}

// ListenAndServe accepts any arbitrary number of `route` functions that
// register an API route with the HTTP serve mux.
func (s WebServer) ListenAndServe(port string, routes ...func(*http.ServeMux)) {
	p := makePortString(port)

	handler := http.NewServeMux()

	for _, addRoute := range routes {
		addRoute(handler)
	}

	s.logger.Infof("Starting web events service on %s", p)

	err := http.ListenAndServe(p, handler)
	if err != nil {
		s.errs <- errors.Wrap(err, "failed to serve http")
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
	RecentLogogram       *utils.FixedQueue[chat.LogogramIteration]
	RecentSpecifications *utils.FixedQueue[memory.SpecificationGeneration]
	RecentUsedWords      *utils.FixedQueue[chat.AgentDictionaryWordsDetectionResponse]
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

	usedWords, err := utils.NewFixedQueue[chat.AgentDictionaryWordsDetectionResponse](2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make recent used words queue")
	}

	rl, err := utils.NewFixedQueue[chat.LogogramIteration](2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make recent logogram queue")
	}

	initData := &InitialData{
		RecentMessages:       recentMessagesQueue,
		RecentGenerations:    rg,
		RecentSpecifications: specs,
		RecentUsedWords:      usedWords,
		RecentLogogram:       rl,
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
	errMsg := "failed serving ChatClient"

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

			_ = rc.Flush()

		case <-c.done:
			return nil
		}
	}
}

type HttpRequestClient[T any] struct {
	hc *http.Client
	u  *url.URL
	l  *log.Logger
}

func NewHttpRequestClient[T any](u *url.URL, logger *log.Logger) (*HttpRequestClient[T], error) {
	if u == nil {
		return nil, errors.New("url cannot be nil")
	}

	hc := &http.Client{Timeout: 0}

	return &HttpRequestClient[T]{
		hc: hc,
		u:  u,
		l:  logger,
	}, nil
}

// PreparePost prepares a body for a POST request, then returns a function that
// executes that POST request.
func (h HttpRequestClient[T]) PreparePost(body any) (func(context.Context) (T, error), error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare http request body")
	}

	return func(ctx context.Context) (T, error) {
		var v T

		req, err := http.NewRequestWithContext(
			ctx, http.MethodPost, h.u.String(),
			bytes.NewReader(b),
		)
		if err != nil {
			return v, errors.Wrap(err, "failed to create request")
		}

		res, err := h.hc.Do(req)
		if err != nil {
			return v, errors.Wrap(err, "failed to send request")
		}

		defer res.Body.Close()

		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return v, errors.Wrap(err, "failed to read response body")
		}

		err = json.Unmarshal(resBody, &v)
		if err != nil {
			return v, errors.Wrap(err, "failed to unmarshal response body")
		}

		return v, nil
	}, nil
}
