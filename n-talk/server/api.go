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

type channels struct {
	// messagePool contains messages that need to be sent to the clients
	// connected to the server.
	messagePool memory.MessageChannel

	// systemLayerMessagePool contains messages that are destined for the server.
	systemLayerMessagePool memory.MessageChannel

	// exchanged is a signaling channel to detect whether an exchange
	// between two clients has been completed.
	exchanged chan bool

	// hitJoinTarget signals when the number of wanted client joins is filled.
	hitJoinTarget chan struct{}
}

type Server struct {
	chat.UnimplementedChatServiceServer
	clients  *clientele
	mu       sync.Mutex
	name     chat.Name
	logger   *log.Logger
	memory   LocalMemory
	channels channels
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
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

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
	var pbMsg *chat.Message

	disconnected := false

	for !disconnected {

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

			s.logger.Info(
				"client disconnected",
				"client",
				client.name,
				"reason",
				err,
			)

		} else {

			// Strip away any `Command`

			fromSender := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer,
			)

			s.channels.messagePool <- *fromSender

			// If all is well, save the message to transcript.

			s.logger.Infof("%s: %s", fromSender.Sender, fromSender.Text)

			// Emit done signal for evolution function.
			s.channels.exchanged <- true
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// router routes messages to clients based on message parameters. It listens
// on the `messagePool` channel for messages.
func (s *Server) router() {
	for msg := range s.channels.messagePool {

		// Inspect sender.

		if msg.Layer == chat.SystemLayer && msg.Sender == chat.
			SystemName && msg.
			Receiver == "SERVER" {
			s.channels.systemLayerMessagePool <- msg
			continue
		}

		err := s.saveToTranscript(context.TODO(), &msg)
		if err != nil {
			s.logger.Errorf("error saving to transcript: %v", err)
			continue
		}

		err = s.broadcast(&msg)
		if err != nil {
			s.logger.Errorf("%v", err)
		}

	}
}

// forward forwards a message `msg` to a client. The client should exist in
// the list of clients maintaining an active connection. routeMessage returns
// an error if the client does not exist.
func (s *Server) forward(msg *memory.Message) error {
	c, err := s.getClientByName(msg.Receiver)
	if err != nil {
		return err
	}

	err = c.Send(msg, msg.Command)
	if err != nil {
		return err
	}

	return nil
}

// broadcast forwards a message `msg` to all clients on a layer, excluding the
// sender. If a message has peers to send to, the message will not be broadcast
// across the entire layer but only broadcasted to the peer in question.
func (s *Server) broadcast(msg *memory.Message) error {
	var err error

	if msg.Receiver != "" {
		err = s.forward(msg)
		if err != nil {
			return err
		}

		return nil
	}

	clients := s.getClientsByLayer(msg.Layer)

	s.logger.Debugf("broadcast message to all clients on layer %s", msg.Layer)

	for _, v := range clients {
		if v.name == msg.Sender {
			continue
		}

		err = v.Send(msg, msg.Command)
		if err != nil {
			return fmt.Errorf("error sending message to client: %v", err)
		}
	}

	return nil
}

// sendCommand issues a command to a client.
func (s *Server) sendCommand(
	command commands.Command,
	to *client,
	content ...chat.Content,
) error {
	msg := memory.NewMessage(memory.UserRole, "command")

	if len(content) > 0 {
		msg.Text = content[0]
	}

	msg.Command = command
	msg.Sender = s.name
	msg.Receiver = to.name

	err := to.Send(&msg, msg.Command)
	if err != nil {
		return err
	}

	s.logger.Debugf("Sent %s to %s", msg.Command, msg.Receiver)

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
