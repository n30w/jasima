package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/n30w/jasima/memory"
)

func (s *ConlangServer) ListenAndServeWebEvents(errs chan<- error) {
	port := ":7070"
	handler := http.NewServeMux()

	handler.HandleFunc("/time", s.sseTime)
	handler.HandleFunc("/events", addCORSHeaders(s.sseChat))

	s.logger.Infof("Starting web events server on %s", port)

	err := http.ListenAndServe(port, handler)
	if err != nil {
		errs <- err
		return
	}
}

func (s *ConlangServer) sseTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// You may need this locally for CORS requests
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for client disconnection
	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)
	t := time.NewTicker(time.Second)

	defer t.Stop()

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
				"data: The time is %s\n\n",
				time.Now().Format(time.UnixDate),
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
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// You may need this locally for CORS requests
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for client disconnection
	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)
	var msg memory.Message

	for {
		select {
		case <-clientGone:
			fmt.Println("Client disconnected")
			return
		case msg = <-s.channels.eventsMessagePool:
			data, err := json.Marshal(msg)
			if err != nil {
				s.logger.Printf("error marshalling message: %s", err)
				return
			}

			// Send an event to the client
			// Here we send only the "data" field, but there are few others
			_, err = fmt.Fprintf(
				w,
				"data: %s\n\n",
				string(data),
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

func addCORSHeaders(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set http headers required for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// You may need this locally for CORS requests
		w.Header().Set("Access-Control-Allow-Origin", "*")

		f(w, r)
	}
}
