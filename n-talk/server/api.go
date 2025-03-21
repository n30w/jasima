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

type Server struct {
	pb.UnimplementedChatServiceServer
	clients    map[string]*client
	mu         sync.Mutex
	serverName string
	logger     *log.Logger
}

func NewServer(name string, l *log.Logger) *Server {
	return &Server{
		clients:    make(map[string]*client),
		serverName: name,
		logger:     l,
	}
}

func (s *Server) ListenAndServe(errors chan<- error) {
	protocol := "tcp"
	port := ":50051"

	lis, err := net.Listen(protocol, port)
	if err != nil {
		errors <- err
		return
	}

	s.logger.Debugf("listener using %s%s", protocol, port)

	grpcServer := grpc.NewServer()

	s.logger.Debug("gRPC server created")

	pb.RegisterChatServiceServer(grpcServer, s)

	s.logger.Debug("registered server with gRPC service")

	err = grpcServer.Serve(lis)
	if err != nil {
		errors <- err
		return
	}
}

// Chat is called by the `client`. The lifetime of this function is for as
// long as the client using this function is connected.
func (s *Server) Chat(stream pb.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}

	client, err := NewClient(stream, firstMsg.Sender, firstMsg.Content)
	if err != nil {
		return err
	}

	s.connect(client)

	s.logger.Info("client connected", "client", client.String())

	// if clientName == "SYSTEM" {
	// 	s.logger.Println("SYSTEM agent online.")
	// }

	// Enter an infinite listening session when the client is connected.

	err = s.listen(stream, client)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) listen(stream pb.ChatService_ChatServer, client *client) error {
	var err error
	disconnected := false

	for !disconnected {
		var msg *pb.Message
		msg, err = stream.Recv()

		if err == io.EOF {

			s.disconnect(client)

			s.logger.Info("client disconnected", "client", client.name)

			disconnected = true

		} else if err != nil {

			s.disconnect(client)

			disconnected = true

		} else {

			// Strip away any `Command` that came from a client by making
			// a new pb message.

			fromSender := s.newPbMessage(msg.Sender, msg.Receiver, msg.Content)

			err = s.routeMessage(fromSender)
			if err != nil {
				s.logger.Errorf("%v", err)
				continue
			}

			// If all is well...

			s.logger.Infof("%s: %s", msg.Sender, msg.Content)

			// Save the message for further processing.
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *Server) connect(client *client) {
	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients[client.name] = client
	s.mu.Unlock()
}

func (s *Server) disconnect(client *client) {
	s.mu.Lock()
	delete(s.clients, client.name)
	s.mu.Unlock()
}

func (s *Server) routeMessage(msg *pb.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error

	// If a message is from a system agent.
	if msg.Sender == "SYSTEM" && msg.Receiver == "SERVER" {
		return nil
	}

	destClient, ok := s.clients[msg.Receiver]

	if ok {
		err = destClient.Send(msg)
	} else {
		err = fmt.Errorf("from: %s -> client [%s] not found", msg.Sender, msg.Receiver)
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *Server) newPbMessage(sender, receiver, content string, command ...int32) *pb.Message {
	m := &pb.Message{
		Sender:   sender,
		Receiver: receiver,
		Content:  content,
		Command:  -1,
	}

	if len(command) > 0 {
		m.Command = command[0]
		m.Sender = s.serverName
	}

	return m
}
