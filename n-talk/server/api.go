package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"text/template"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedChatServiceServer
	clients    map[string]*client
	mu         sync.Mutex
	serverName string
	logger     *log.Logger
	memory     ServerMemoryService
}

func NewServer(name string, l *log.Logger, m ServerMemoryService) *Server {
	return &Server{
		clients:    make(map[string]*client),
		serverName: name,
		logger:     l,
		memory:     m,
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

	client, err := s.initClient(stream, firstMsg)
	if err != nil {
		return err
	}

	// Enter an infinite listening session when the client is connected.
	// Each client receives their own context.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = s.listen(ctx, stream, client)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) initLayer(ctx context.Context, stream pb.ChatService_ChatServer, client *client) error {
	return s.listen(ctx, stream, client)
}

// listen is called when a client connection with `Chat` has already been
// established. It disconnects clients when they error or when they disconnect
// from the server. It also calls `routeMessage` when a message is received
// from the connected client.
func (s *Server) listen(ctx context.Context, stream pb.ChatService_ChatServer, client *client) error {
	var err error
	disconnected := false

	for !disconnected {
		var msg *pb.Message

		// Wait for a message to come in from the client. This is a blocking call.

		msg, err = stream.Recv()

		if err == io.EOF {

			s.removeClient(client)

			s.logger.Info("client disconnected", "client", client.name)

			disconnected = true

		} else if err != nil {

			s.removeClient(client)

			disconnected = true

		} else {

			// Strip away any `Command` that came from a client by making
			// a new pb message.

			fromSender := s.newPbMessage(msg.Sender, msg.Receiver, msg.Content)

			err = s.handleMessage(fromSender)
			if err != nil {
				s.logger.Errorf("%v", err)
				continue
			}

			// If all is well save the message to transcript.

			err = s.saveToTranscript(ctx, fromSender)
			if err != nil {
				s.logger.Errorf("error saving to transcript: %v", err)
				continue
			}

			s.logger.Infof("%s: %s", msg.Sender, msg.Content)

			// Save the message for further processing.
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// saveToTranscript saves a message to the server's memory storage.
func (s *Server) saveToTranscript(ctx context.Context, msg *pb.Message) error {
	m := memory.NewMessage(memory.UserRole, msg.Content)
	m.Sender = msg.Sender

	err := s.memory.Save(ctx, m)
	if err != nil {
		return err
	}

	return nil
}

// initClient initializes a client connection and adds the client to the list
// of clients currently maintaining a connection.
func (s *Server) initClient(stream pb.ChatService_ChatServer, msg *pb.Message) (*client, error) {
	client, err := NewClient(stream, msg.Sender, msg.Content)
	if err != nil {
		return nil, err
	}

	s.addClient(client)

	s.logger.Info("client connected", "client", client.String())

	return client, nil
}

// addClient adds a client to the list of clients that maintain an active
// connection to the server.
func (s *Server) addClient(client *client) {
	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients[client.name] = client
	s.mu.Unlock()
}

// removeClient removes a client from the list of clients that maintain an
// active connection to the server.
func (s *Server) removeClient(client *client) {
	s.mu.Lock()
	delete(s.clients, client.name)
	s.mu.Unlock()
}

// handleMessage decides what happens to a message, based on sender, receiver,
// and the state of the layer.
func (s *Server) handleMessage(msg *pb.Message) error {
	// If a message is from a system agent.

	if msg.Sender == "SYSTEM" && msg.Receiver == "SERVER" {
		return nil
	}

	err := s.routeMessage(msg)
	if err != nil {
		return err
	}

	return nil
}

// routeMessage sends a message `msg` to a client. The client should exist in
// the list of clients maintaining an active connection. routeMessage returns
// an error if the client does not exist.
func (s *Server) routeMessage(msg *pb.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error

	destClient, ok := s.clients[msg.Receiver]

	if ok {
		err = destClient.Send(msg)
	} else {
		err = fmt.Errorf("from: %s -> client [%s] not found", msg.Sender, msg.Receiver)
	}

	return err
}

// newPbMessage constructs a new protobuf Message.
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

type serverMemory struct {
	MemoryService
}

// String serializes all memories into a string.
func (s serverMemory) String() string {
	var builder strings.Builder

	t := template.New("t1")
	t, _ = t.Parse("{{.Sender}}: {{.Text}}\n")

	memories, _ := s.Retrieve(context.Background(), "", 0)

	for _, v := range memories {
		var buff bytes.Buffer
		t.Execute(&buff, v)
		builder.Write(buff.Bytes())
	}

	return builder.String()
}
