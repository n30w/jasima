package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/n30w/jasima/memory"
)

func (s *ConlangServer) ListenAndServeWebEvents(
	port string,
	errs chan<- error,
) {
	p := makePortString(port)

	handler := http.NewServeMux()

	handler.HandleFunc("/time", s.sseTime)
	handler.HandleFunc("/events", s.sseChat)
	handler.HandleFunc("/test/chat", s.sseChat)

	s.logger.Infof("Starting web events service on %s", p)

	err := http.ListenAndServe(p, handler)
	if err != nil {
		errs <- err
		return
	}
}

func (s *ConlangServer) sseTime(w http.ResponseWriter, r *http.Request) {
	addEventHeaders(w)

	// Create a channel for client disconnection
	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)
	t := time.NewTicker(time.Second)

	defer t.Stop()

	clientChan := make(chan memory.Message, 10)

	s.mu.Lock()
	s.webClients[clientChan] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.webClients, clientChan)
		s.mu.Unlock()
		close(clientChan)
	}()

	for {
		select {
		case <-clientGone:
			fmt.Println("Client disconnected")
			return
		case <-t.C:
			// Send an event to the client
			// Here we send only the "data" field, but there are few others
			_, err := fmt.Fprintf(
				w,
				makeDataString(time.Now().Format(time.UnixDate)),
			)
			if err != nil {
				return
			}
			err = rc.Flush()
			if err != nil {
				return
			}
		}
	}
}

func (s *ConlangServer) sseChat(w http.ResponseWriter, r *http.Request) {
	addEventHeaders(w)

	s.logger.Infof("Web client connected %s", r.RemoteAddr)

	// Create a channel for client disconnection
	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)
	var msg memory.Message

	clientChan := make(chan memory.Message, 10)

	s.mu.Lock()
	s.webClients[clientChan] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.webClients, clientChan)
		s.mu.Unlock()
		close(clientChan)
	}()

	d, _ := json.Marshal(s.mostRecentEvent)

	fmt.Fprintf(w, makeDataString(string(d)))

	err := rc.Flush()
	if err != nil {
		return
	}

	for {
		select {
		case <-clientGone:
			s.logger.Info("Web client disconnected")
			return
		case msg = <-clientChan:
			data, err := json.Marshal(msg)
			if err != nil {
				s.logger.Printf("error marshalling message: %s", err)
				return
			}

			_, err = fmt.Fprintf(
				w,
				makeDataString(string(data)),
			)
			if err != nil {
				s.logger.Errorf("error writing message: %s", err)
				return
			}

			err = rc.Flush()
			if err != nil {
				s.logger.Errorf("error flushing response controller: %s", err)
				return
			}
		}
	}
}

func addEventHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func makeDataString(s string) string {
	return fmt.Sprintf("data: %s\n\n", s)
}
