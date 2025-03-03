package main

import (
	"fmt"
	"io"
	"net"
	"sync"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

type client struct {
	stream pb.ChatService_ChatServer
	name   string
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

	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients[clientName] = &client{stream: stream, name: clientName}
	s.mu.Unlock()

	log.Info("Client connected", "client", clientName)

	// Enter an infinite listening session when the client is connected.

	for {
		msg, err := stream.Recv()

		if err == io.EOF {

			// Delete the client from the list of current clients.

			s.mu.Lock()
			delete(s.clients, clientName)
			s.mu.Unlock()

			log.Info("Client disconnected\n", "client", clientName)

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

	if ok {

		if err := destClient.stream.Send(msg); err != nil {
			log.Error("Failed to send message to %s: %v\n", msg.Receiver, err)
		} else {
			log.Info("Routed message", "sender", msg.Sender, "recipient", msg.Receiver)
		}

	} else {

		log.Warnf("Client %s not found\n", msg.Receiver)

		if sender, ok := s.clients[msg.Sender]; ok {
			sender.stream.Send(&pb.Message{Sender: "Server", Content: fmt.Sprintf("Client %s not found", msg.Receiver)})
		}

	}
}

func main() {

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
