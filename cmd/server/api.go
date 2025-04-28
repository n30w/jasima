package main

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/agent"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

type channels struct {
	// messagePool contains messages that need to be sent to the clients
	// connected to the server.
	messagePool chan *chat.Message

	// systemLayerMessagePool contains messages that are destined for the server.
	systemLayerMessagePool memory.MessageChannel

	// eventsMessagePool contains messages and data that are to be published as
	// web events.
	eventsMessagePool memory.MessageChannel

	// exchanged is a signaling channel to detect whether an exchange
	// between two clients has been completed.
	exchanged chan bool
}

type Server struct {
	chat.UnimplementedChatServiceServer
	clients  *clientele
	mu       sync.Mutex
	name     chat.Name
	logger   *log.Logger
	memory   MemoryService
	channels channels

	// messages contains all messages sent back and forth. Used for debugging.
	messages []memory.Message

	// listening determines whether the server will operate on messages,
	// whether it be through routing, saving, etc.
	listening bool
}

func (s *Server) ListenAndServeRPC(protocol, port string, errs chan<- error) {
	p := makePortString(port)

	lis, err := net.Listen(protocol, p)
	if err != nil {
		errs <- err
		return
	}

	grpcServer := grpc.NewServer()

	chat.RegisterChatServiceServer(grpcServer, s)

	s.logger.Debugf("gRPC servicing @ %s%s", protocol, p)

	err = grpcServer.Serve(lis)
	if err != nil {
		errs <- err
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
	// Each client receives their own context. `listen` is a blocking call.

	err = s.listen(stream)

	s.removeClient(c)

	if err == io.EOF {
		s.logger.Info("Client disconnected", "client", c.name)
	} else if err != nil {
		return errors.Wrap(err, "unexpected client disconnection")
	}

	return nil
}

// listen is called when a client connection with `Chat` has already been
// established. It disconnects clients when they error or when they disconnect
// from the server. It also calls `routeMessage` when a message is received
// from the connected client.
func (s *Server) listen(
	stream chat.ChatService_ChatServer,
) error {
	var (
		err          error
		msg          *chat.Message
		disconnected bool
	)

	for !disconnected {

		// Wait for a message to come in from the client. This is a blocking call.

		msg, err = stream.Recv()
		if err != nil {
			disconnected = true
			continue
		}

		// Discard if the server is not listening.

		if !s.listening {
			continue
		}

		select {
		case s.channels.messagePool <- msg:
		default:
		}
	}

	if err != nil {
		return err
	}

	return nil
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
			return errors.Wrap(err, "message has no receiver")
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
			return errors.Wrapf(
				err,
				"failed to broadcast message on layer %s",
				msg.Layer,
			)
		}
	}

	return nil
}

// sendCommand issues a command to a client.
func (s *Server) sendCommand(
	command agent.Command,
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
		return errors.Wrap(err, "failed to send command")
	}

	s.logger.Debugf("Sent %s to %s", msg.Command, msg.Receiver)

	return nil
}

func saveMessageTo(
	ctx context.Context,
	mem MemoryService,
	msg memory.Message,
) error {
	msg.Role = memory.UserRole
	err := mem.Save(ctx, msg)
	if err != nil {
		return errors.Wrap(err, "failed to save message")
	}

	return nil
}
