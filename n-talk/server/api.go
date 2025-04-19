package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	chat "codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/commands"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

type Server struct {
	chat.UnimplementedChatServiceServer
	clients *clientele
	mu      sync.Mutex
	name    chat.Name
	logger  *log.Logger
	memory  LocalMemory

	// exchangeComplete is a signaling channel to detect whether
	// an exchange between two clients has been completed.
	exchangeComplete chan bool
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

	chat.RegisterChatServiceServer(grpcServer, s)

	s.logger.Debug("registered server with gRPC service")

	err = grpcServer.Serve(lis)
	if err != nil {
		errors <- err
		return
	}
}

// Chat is called by the `client`. The lifetime of this function is for as
// long as the client using this function is connected.
func (s *Server) Chat(stream chat.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}

	c, err := s.initClient(stream, firstMsg)
	if err != nil {
		return err
	}

	// Enter an infinite listening session when the client is connected.
	// Each client receives their own context.

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	err = s.listen(ctx, stream, c)
	if err != nil {
		return err
	}

	return nil
}

// listen is called when a client connection with `Chat` has already been
// established. It disconnects clients when they error or when they disconnect
// from the server. It also calls `routeMessage` when a message is received
// from the connected client.
func (s *Server) listen(
	ctx context.Context,
	stream chat.ChatService_ChatServer,
	client *client,
) error {
	var err error
	disconnected := false

	for !disconnected {
		var pbMsg *chat.Message

		// Wait for a message to come in from the client. This is a blocking call.

		pbMsg, err = stream.Recv()

		// Translate the pbMsg into our custom type.

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

			fromSender := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer,
			)

			err = s.handleMessage(fromSender)
			if err != nil {
				s.logger.Errorf("%v", err)
				continue
			}

			// If all is well, save the message to transcript.

			err = s.saveToTranscript(ctx, fromSender)
			if err != nil {
				s.logger.Errorf("error saving to transcript: %v", err)
				continue
			}

			s.logger.Infof("%s: %s", fromSender.Sender, fromSender.Text)

			// Emit done signal for evolution function.
			s.exchangeComplete <- true
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// saveToTranscript saves a message to the server's memory storage.
func (s *Server) saveToTranscript(
	ctx context.Context,
	msg *memory.Message,
) error {
	msg.Role = memory.UserRole

	err := s.memory.Save(ctx, *msg)
	if err != nil {
		return err
	}

	return nil
}

// initClient initializes a client connection and adds the client to the list
// of clients currently maintaining a connection.
func (s *Server) initClient(
	stream chat.ChatService_ChatServer,
	msg *chat.Message,
) (*client, error) {
	client, err := newClient(stream, msg.Sender, msg.Content, msg.Layer)
	if err != nil {
		return nil, err
	}

	s.addClient(client)

	s.logger.Info(
		"Client connected",
		"client",
		client.String(),
		"Layer",
		client.layer,
	)

	return client, nil
}

// addClient adds a client to the list of clients that maintain an active
// connection to the server.
func (s *Server) addClient(client *client) {
	// Add the client to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
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

// getClientsByLayer retrieves all the clients of a Layer and returns them
// in an array of pointers to those clients.
func (s *Server) getClientsByLayer(layer chat.Layer) Clients {
	var c Clients

	s.mu.Lock()
	c = s.clients.byLayer(layer)
	s.mu.Unlock()

	return c
}

func (s *Server) getClientByName(name chat.Name) (*client, error) {
	var c *client
	var ok bool

	c, ok = s.clients.byName(name)

	if !ok {
		return nil, fmt.Errorf("client with name: '%s' not found", name)
	}

	return c, nil
}

// handleMessage decides what happens to a message, based on sender, receiver,
// and the state of the Layer.
func (s *Server) handleMessage(msg *memory.Message) error {
	// If a message is from a system agent.

	if msg.Sender == chat.SystemName && msg.Receiver == "SERVER" {
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
func (s *Server) routeMessage(msg *memory.Message) error {
	// Lock for the entirety of this function, as we use the client for
	// the lifetime of this function.
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error

	// destClient, ok := s.clients.byNameMap[msg.Receiver]
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

// SendCommand issues a command to a client.
func (s *Server) SendCommand(command commands.Command, to *client) error {
	msg := memory.NewMessage(memory.ChatRole(0), "command")

	msg.Command = command
	msg.Sender = s.name
	msg.Receiver = to.name

	err := to.Send(&msg)
	if err != nil {
		return err
	}

	return nil
}
