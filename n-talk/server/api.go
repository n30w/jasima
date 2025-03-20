package main

import (
	"fmt"
	"io"
	"log"
	"sync"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
)

type client struct {
	stream pb.ChatService_ChatServer
	name   string
	model  string
}

type chatServer struct {
	pb.UnimplementedChatServiceServer
	clients    map[string]*client
	mu         sync.Mutex
	serverName string
}

func newChatServer(name string) *chatServer {
	return &chatServer{
		clients:    make(map[string]*client),
		serverName: name,
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

	if clientName == "SYSTEM" {
		log.Println("SYSTEM agent online.")
	}

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

		err = s.routeMessage(msg)
		if err != nil {
			log.Printf("%v", err)
		}
	}
}

func (s *chatServer) routeMessage(msg *pb.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If a message is from a system agent.
	if msg.Sender == "SYSTEM" && msg.Receiver == "SERVER" {
		return nil
	}

	destClient, ok := s.clients[msg.Receiver]
	originClient := s.clients[msg.Sender]

	if ok {
		err := destClient.stream.Send(msg)
		if err != nil {
			log.Printf("Failed to send message to %s: %v\n", msg.Receiver, err)
		} else {
			log.Printf("%s [%s]: %s", originClient.name, originClient.model, msg.Content)

			// Also save the message for further processing.
		}
	} else {

		log.Printf("Client %s not found // From: %s\n", msg.Receiver, msg.Sender)

		if sender, ok := s.clients[msg.Sender]; ok {
			content := fmt.Sprintf("Client %s not found", msg.Receiver)
			err := sender.stream.Send(s.NewPbMessage(msg.Receiver, content))
			if err != nil {
				log.Printf("%v", err)
			}
		}
	}

	return nil
}

func (s *chatServer) NewPbMessage(receiver, content string) *pb.Message {
	return &pb.Message{
		Sender:   s.serverName,
		Receiver: receiver,
		Content:  content,
	}
}
