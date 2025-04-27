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
	messagePool memory.MessageChannel

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

func (s *Server) ListenAndServeRouter(errs chan<- error) {
	protocol := "tcp"
	port := ":50051"

	lis, err := net.Listen(protocol, port)
	if err != nil {
		errs <- err
		return
	}

	s.logger.Debugf("listener using %s%s", protocol, port)

	grpcServer := grpc.NewServer()

	s.logger.Debug("gRPC server created")

	chat.RegisterChatServiceServer(grpcServer, s)

	s.logger.Debug("registered server with gRPC service")

	err = grpcServer.Serve(lis)
	if err != nil {
		errs <- err
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

			// Discard if the server is not listening.

			if !s.listening {
				continue
			}

			msg := memory.NewChatMessage(
				pbMsg.Sender, pbMsg.Receiver,
				pbMsg.Content, pbMsg.Layer,
			)

			select {
			case s.channels.messagePool <- *msg:
				s.logger.Printf("%s: %s", msg.Sender, msg.Text)
			default:
			}
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// router routes messages to different services based on message parameters.
// It listens on the `messagePool` channel for messages.
func (s *Server) router() {
	for msg := range s.channels.messagePool {

		select {
		case s.channels.eventsMessagePool <- msg:
		default:
		}

		var err error

		s.messages = append(s.messages, msg)

		err = saveMessageTo(context.Background(), s.memory, msg)
		if err != nil {
			s.logger.Errorf("failed to save message: %s", err)
		}

		// Route messages for the server that come from the system layer
		// agents.

		if msg.Layer == chat.SystemLayer && msg.Sender == chat.SystemName && msg.
			Receiver == "SERVER" {
			s.channels.systemLayerMessagePool <- msg
			continue
		}

		// If the message is not from the server itself, save it to memory
		// and notify that an exchange has occurred.

		if msg.Sender != "SERVER" {
			err = saveMessageTo(context.Background(), s.memory, msg)
			if err != nil {
				s.logger.Errorf("error saving to transcript: %v", err)
			}

			select {
			case s.channels.exchanged <- true:
			default:
			}
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
