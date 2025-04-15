package server

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
	clients    *clientele
	mu         sync.Mutex
	serverName string
	logger     *log.Logger
	memory     ServerMemoryService

	// exchangeComplete is a signaling channel to detect whether or not
	// an exchange between two clients has been completed.
	exchangeComplete chan bool
}

// func newServer(name string, l *log.Logger, m ServerMemoryService) *Server {
// 	return &Server{
// 		clients: &clientele{
// 			byName:  make(nameToClientsMap),
// 			byLayer: make(layerToNamesMap),
// 		},
// 		serverName:       name,
// 		logger:           l,
// 		memory:           m,
// 		exchangeComplete: make(chan bool),
// 	}
// }

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

			fromSender := s.newPbMessage(msg.Sender, msg.Receiver, msg.Content, msg.Layer)

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

			// Emit done signal for evolution function.
			s.exchangeComplete <- true

			s.logger.Infof("%s: %s", msg.Sender, msg.Content)
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
	client, err := NewClient(stream, msg.Sender, msg.Content, msg.Layer)
	if err != nil {
		return nil, err
	}

	s.addClient(client)

	s.logger.Info("Client connected", "client", client.String(), "layer", client.layer)

	return client, nil
}

// addClient adds a client to the list of clients that maintain an active
// connection to the server.
func (s *Server) addClient(client *client) {
	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	// s.clients.byName[client.name] = client
	s.clients.addByName(client)
	s.clients.addByLayer(client)
	s.mu.Unlock()
}

// removeClient removes a client from the list of clients that maintain an
// active connection to the server.
func (s *Server) removeClient(client *client) {
	s.mu.Lock()
	s.clients.removeByName(client)
	s.clients.removeByLayer(client)
	s.mu.Unlock()
}

func (s *Server) getClientsByLayer(layer int32) []*client {
	var c []*client

	s.mu.Lock()
	c = s.clients.getByLayer(layer)
	s.mu.Unlock()

	return c
}

func (s *Server) getClientByName(name string) (*client, error) {
	var c *client
	var ok bool

	c, ok = s.clients.getByName(name)

	if !ok {
		return nil, fmt.Errorf("client with name: '%s' not found", name)
	}

	return c, nil
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

	// destClient, ok := s.clients.byName[msg.Receiver]
	destClient, err := s.getClientByName(msg.Receiver)
	if err != nil {
		return err
	}

	err = destClient.Send(msg)
	if err != nil {
		return err
	}

	return nil
}

// newPbMessage constructs a new protobuf Message.
func (s *Server) newPbMessage(sender, receiver, content string, layer int32, command ...int32) *pb.Message {
	m := &pb.Message{
		Sender:   sender,
		Receiver: receiver,
		Content:  content,
		Command:  -1,
		Layer:    layer,
	}

	if len(command) > 0 {
		m.Command = command[0]
		m.Sender = s.serverName
	}

	return m
}

// SendCommand issues a command to a client.
func (s *Server) SendCommand(command Command, to *client) error {
	msg := &pb.Message{
		Command: int32(command),
		Layer:   to.layer,
	}

	err := to.Send(msg)
	if err != nil {
		return err
	}

	return nil
}

type ServerMemory struct {
	MemoryService
}

// String serializes all memories into a string.
func (s ServerMemory) String() string {
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
