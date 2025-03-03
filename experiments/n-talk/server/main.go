package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
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

func (s *chatServer) Chat(stream pb.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}
	clientName := firstMsg.Sender
	if clientName == "" {
		return fmt.Errorf("client name cannot be empty")
	}

	s.mu.Lock()
	s.clients[clientName] = &client{stream: stream, name: clientName}
	s.mu.Unlock()

	fmt.Printf("Client %s connected\n", clientName)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			s.mu.Lock()
			delete(s.clients, clientName)
			s.mu.Unlock()
			fmt.Printf("Client %s disconnected\n", clientName)
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

	if destClient, ok := s.clients[msg.Receiver]; ok {
		if err := destClient.stream.Send(msg); err != nil {
			fmt.Printf("Failed to send message to %s: %v\n", msg.Receiver, err)
		} else {
			fmt.Printf("Routed message from %s to %s\n", msg.Sender, msg.Receiver)
		}
	} else {
		fmt.Printf("Client %s not found\n", msg.Receiver)
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
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
