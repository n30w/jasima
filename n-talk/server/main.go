package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"google.golang.org/grpc"
)

type client struct {
	stream pb.ChatService_ChatServer
	name   string
	model  string
}

type chatServer struct {
	pb.UnimplementedChatServiceServer
	clients map[string]*client
	mu      sync.Mutex
}

func newChatServer() *chatServer {
	return &chatServer{
		clients: make(map[string]*client),
	}
}

// Chat is called by the `client`. The lifetime of this function is for as
// long as the client using this function is connected.
func (s *chatServer) Chat(stream pb.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}

	clientName := firstMsg.Sender
	if clientName == "" {
		return fmt.Errorf("client name cannot be empty")
	}

	clientModel := firstMsg.Content

	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients[clientName] = &client{
		stream: stream,
		name:   clientName,
		model:  clientModel,
	}
	s.mu.Unlock()

	log.Println("Client connected", "client", clientName, "model", clientModel)

	// Enter an infinite listening session when the client is connected.

	for {
		msg, err := stream.Recv()

		if err == io.EOF {

			// Delete the client from the list of current clients.

			s.mu.Lock()
			delete(s.clients, clientName)
			s.mu.Unlock()

			log.Println("Client disconnected\n", "client", clientName)

			return nil
		}

		if err != nil {
			return err
		}

		s.routeMessage(msg)
	}
}

func (s *chatServer) routeMessage(msg *pb.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	destClient, ok := s.clients[msg.Receiver]
	originClient := s.clients[msg.Sender]

	if ok {

		if err := destClient.stream.Send(msg); err != nil {
			log.Printf("Failed to send message to %s: %v\n", msg.Receiver, err)
		} else {
			log.Printf("%s [%s]: %s", originClient.name, originClient.model, msg.Content)
			// log.Info("Routed message", "sender", msg.Sender, "recipient", msg.Receiver)
		}

	} else {

		log.Printf("Client %s not found\n", msg.Receiver)

		if sender, ok := s.clients[msg.Sender]; ok {
			sender.stream.Send(&pb.Message{Sender: "Server", Content: fmt.Sprintf("Client %s not found", msg.Receiver)})
		}

	}
}

func main() {
	fn := logOutput()
	defer fn()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	chatServer := newChatServer()
	pb.RegisterChatServiceServer(s, chatServer)

	err = s.Serve(lis)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// https://gist.github.com/jerblack/4b98ba48ed3fb1d9f7544d2b1a1be287
func logOutput() func() {
	logFile := fmt.Sprintf("../outputs/server_log_%s.log", time.Now().Format(time.RFC3339))
	// open file read/write | create if not exist | clear file at open if exists
	f, _ := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)

	// save existing stdout | MultiWriter writes to saved stdout and file
	out := os.Stdout
	mw := io.MultiWriter(out, f)

	// get pipe reader and writer | writes to pipe writer come out pipe reader
	r, w, _ := os.Pipe()

	// replace stdout,stderr with pipe writer | all writes to stdout, stderr will go through pipe instead (fmt.print, log)
	os.Stdout = w
	os.Stderr = w

	// writes with log.Print should also write to mw
	log.SetOutput(mw)

	//create channel to control exit | will block until all copies are finished
	exit := make(chan bool)

	go func() {
		// copy all reads from pipe to multiwriter, which writes to stdout and file
		_, _ = io.Copy(mw, r)
		// when r or w is closed copy will finish and true will be sent to channel
		exit <- true
	}()

	// function to be deferred in main until program exits
	return func() {
		// close writer then block on exit channel | this will let mw finish writing before the program exits
		_ = w.Close()
		<-exit
		// close file after all writes have finished
		_ = f.Close()
	}
}
