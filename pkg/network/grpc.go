package network

import (
	"context"
	"fmt"
	"io"
	"sync"

	"google.golang.org/grpc/credentials/insecure"

	"codeberg.org/n30w/jasima/pkg/agent"
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type channels struct {
	// ToClients contains messages that need to be sent to the clients
	// connected to the server.
	ToClients chan *chat.Message

	// ToServer contains messages that are destined for the server.
	ToServer memory.MessageChannel
}

func (c channels) Teardown() {
	if c.ToClients != nil {
		close(c.ToClients)
	}
	if c.ToServer != nil {
		close(c.ToServer)
	}
}

type ChatServer struct {
	chat.UnimplementedChatServiceServer
	clients    *clientele
	mu         sync.Mutex
	logger     *log.Logger
	Channel    *channels
	grpcServer *grpc.Server
	*ServerBase

	// listening determines whether the server will operate on messages,
	// whether it be through routing, saving, etc.
	Listening bool
}

func NewChatServer(
	logger *log.Logger,
	errs chan<- error,
	opts ...func(*config),
) (*ChatServer, error) {
	clients := &clientele{
		byNameMap:  make(nameToClientsMap),
		byLayerMap: make(layerToNamesMap),
	}

	chs := &channels{
		ToClients: make(chan *chat.Message, 100),
		ToServer:  make(memory.MessageChannel, 100),
	}

	cfg := newConfigWithOpts(defaultChatServerConfig, opts...)

	b, err := newServerBase(cfg, errs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize grpc server")
	}

	cs := &ChatServer{
		Listening:  true,
		Channel:    chs,
		clients:    clients,
		logger:     logger,
		ServerBase: b,
		grpcServer: grpc.NewServer(),
	}

	chat.RegisterChatServiceServer(cs.grpcServer, cs)

	return cs, nil
}

func (s *ChatServer) ListenAndServe(ctx context.Context) {
	serverCtx, serverCancel := context.WithCancel(ctx)

	defer serverCancel()

	go func() {
		defer serverCancel()
		s.logger.Infof("Starting gRPC service on %s %s", s.config.protocol, s.config.addr)
		err := s.grpcServer.Serve(s.listener)
		if err != nil {
			s.errs <- err
		}
	}()

	<-serverCtx.Done()

	err := s.Shutdown()
	if err != nil {
		s.errs <- err
	}
}

func (s *ChatServer) Shutdown() error {
	// s.grpcServer.GracefulStop()

	s.Channel.Teardown()

	s.grpcServer.Stop()

	s.logger.Infof("gRPC server shut down successfully")

	return nil
}

// Chat is called by the `ChatClient`. The lifetime of this function is for as
// long as the ChatClient using this function is connected.
func (s *ChatServer) Chat(stream chat.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}

	c, err := s.initClient(stream, firstMsg)
	if err != nil {
		return err
	}

	// Enter an infinite listening session when the ChatClient is connected.
	// Each ChatClient receives their own context. `listen` is a blocking call.

	err = s.listen(c)

	s.removeClient(c)

	if err == io.EOF {
		s.logger.Info("Client disconnected", "ChatClient", c.Name)
	} else if err != nil {
		return errors.Wrap(err, "unexpected ChatClient disconnection")
	}

	return nil
}

// listen is called when a ChatClient connection with `Chat` has already been
// established. It disconnects clients when they error or when they disconnect
// from the server. It also calls `routeMessage` when a message is received
// from the connected ChatClient.
func (s *ChatServer) listen(c *ChatClient) error {
	var (
		err error
		msg *chat.Message
	)

	streamCtx := c.stream.Context()

	for {
		select {
		case <-streamCtx.Done():
			return streamCtx.Err()
		default:
			msg, err = c.stream.Recv()
			if err != nil {
				return err
			}

			s.Channel.ToClients <- msg
		}
	}
}

// Broadcast forwards a message `msg` to all clients on a layer, excluding the
// sender. If a message has peers to send to, the message will not be broadcast
// across the entire layer but only broadcasted to the peer in question.
func (s *ChatServer) Broadcast(msg *memory.Message) error {
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
		if v.Name == msg.Sender {
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

// forward forwards a message `msg` to a ChatClient. The ChatClient should exist in
// the list of clients maintaining an active connection. routeMessage returns
// an error if the ChatClient does not exist.
func (s *ChatServer) forward(msg *memory.Message) error {
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

func (s *ChatServer) TotalClients() int {
	return s.clients.total
}

// initClient initializes a ChatClient connection and adds the ChatClient to the
// list of clients currently maintaining a connection.
func (s *ChatServer) initClient(
	stream chat.ChatService_ChatServer,
	msg *chat.Message,
) (*ChatClient, error) {
	c, err := newChatClient(stream, msg.Sender, msg.Layer)
	if err != nil {
		return nil, err
	}

	s.addClient(c)

	s.logger.Info(
		"Client connected",
		"client",
		c.String(),
		"layer",
		c.layer,
	)

	return c, nil
}

func (s *ChatServer) AddClient(c *ChatClient) {
	s.addClient(c)
}

func (s *ChatServer) RemoveClient(c *ChatClient) {
	s.removeClient(c)
}

func (s *ChatServer) GetClientsByLayer(layer chat.Layer) []*ChatClient {
	return s.getClientsByLayer(layer)
}

func (s *ChatServer) GetClientByName(name chat.Name) (*ChatClient, error) {
	return s.getClientByName(name)
}

// addClient adds a ChatClient to the list of clients that maintain an active
// connection to the server.
func (s *ChatServer) addClient(client *ChatClient) {
	// Add the ChatClient to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients.addByName(client)
	s.clients.addByLayer(client)
	s.clients.total++
	s.mu.Unlock()
}

// removeClient removes a ChatClient from the list of clients that maintain an
// active connection to the server.
func (s *ChatServer) removeClient(client *ChatClient) {
	s.mu.Lock()
	s.clients.removeByName(client)
	s.clients.removeByLayer(client)
	s.clients.total--
	s.mu.Unlock()
}

// getClientsByLayer retrieves all the clients of a Layer and returns them
// in an array of pointers to those clients.
func (s *ChatServer) getClientsByLayer(layer chat.Layer) []*ChatClient {
	var c []*ChatClient

	s.mu.Lock()
	c = s.clients.byLayer(layer)
	s.mu.Unlock()

	return c
}

func (s *ChatServer) getClientByName(name chat.Name) (*ChatClient, error) {
	var c *ChatClient
	var ok bool

	c, ok = s.clients.byName(name)

	if !ok {
		return nil, fmt.Errorf("ChatClient with name: '%s' not found", name)
	}

	return c, nil
}

// ChatClient represents a client connected to the server.
type ChatClient struct {
	Name     chat.Name
	stream   chat.ChatService_ChatServer
	layer    chat.Layer
	channels map[chan *chat.Message]struct{}
	mu       sync.Mutex
}

func newChatClient(
	stream chat.ChatService_ChatServer,
	name string,
	l int32,
) (*ChatClient, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	c := &ChatClient{
		stream:   stream,
		Name:     chat.Name(name),
		layer:    chat.Layer(l),
		mu:       sync.Mutex{},
		channels: make(map[chan *chat.Message]struct{}),
	}

	return c, nil
}

func (c *ChatClient) SendWithChannel(
	msg *memory.Message,
	command ...agent.Command,
) (chan *chat.Message, error) {
	pbMsg := chat.NewPbMessage(
		msg.Sender, msg.Receiver, msg.Text, msg.Layer,
		command...,
	)

	ch := make(chan *chat.Message, 10)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.channels[ch] = struct{}{}

	return ch, c.send(pbMsg)
}

func (c *ChatClient) Send(msg *memory.Message, command ...agent.Command) error {
	pbMsg := chat.NewPbMessage(
		msg.Sender, msg.Receiver, msg.Text, msg.Layer,
		command...,
	)

	return c.send(pbMsg)
}

func (c *ChatClient) send(msg *chat.Message) error {
	err := c.stream.Send(msg)
	if err != nil {
		return fmt.Errorf(
			"failed to send message to: %s: %v",
			msg.Receiver,
			err,
		)
	}

	return nil
}

func (c *ChatClient) String() string {
	return c.Name.String()
}

type (
	namesMap         map[chat.Name]struct{}
	layerToNamesMap  map[chat.Layer]namesMap
	nameToClientsMap map[chat.Name]*ChatClient
)

type clientele struct {
	byNameMap  nameToClientsMap
	byLayerMap layerToNamesMap
	total      int
}

func (ct *clientele) addByName(c *ChatClient) {
	ct.byNameMap[c.Name] = c
}

func (ct *clientele) removeByName(c *ChatClient) {
	delete(ct.byNameMap, c.Name)
}

func (ct *clientele) addByLayer(c *ChatClient) {
	_, ok := ct.byLayerMap[c.layer]
	if !ok {
		ct.byLayerMap[c.layer] = make(namesMap)
	}

	ct.byLayerMap[c.layer][c.Name] = struct{}{}
}

func (ct *clientele) removeByLayer(c *ChatClient) {
	delete(ct.byLayerMap[c.layer], c.Name)
}

func (ct *clientele) byName(n chat.Name) (*ChatClient, bool) {
	c, ok := ct.byNameMap[n]
	return c, ok
}

// byLayer receives a Layer parameter to retrieve an `n` list of clients with
// that specified Layer.
func (ct *clientele) byLayer(layer chat.Layer) []*ChatClient {
	clients := make([]*ChatClient, 0)
	l := ct.byLayerMap[layer]

	for k := range l {
		c, ok := ct.byName(k)
		if ok {
			clients = append(clients, c)
		}
	}

	return clients
}

// ChatClientService defines a facade for an agent to use to communicate
// chat messages to and from a server.
type ChatClientService struct {
	conn       grpc.BidiStreamingClient[chat.Message, chat.Message]
	grpcClient *grpc.ClientConn
	channel    *channels
}

func NewChatClientService(
	ctx context.Context,
	url string,
	inbound chan *chat.Message,
) (*ChatClientService, error) {
	grpcClient, err := grpc.NewClient(
		url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes the initial connection to the server.
	conn, err := chat.NewChatServiceClient(grpcClient).Chat(ctx)
	if err != nil {
		return nil, err
	}

	return &ChatClientService{
		conn:       conn,
		grpcClient: grpcClient,
		channel: &channels{
			ToClients: inbound,
		},
	}, nil
}

func (c *ChatClientService) Send(msg *chat.Message) error {
	return c.conn.Send(msg)
}

func (c *ChatClientService) Receive() error {
	return c.listen()
}

func (c *ChatClientService) listen() error {
	var (
		err          error
		msg          *chat.Message
		disconnected bool
	)

	for !disconnected {
		msg, err = c.conn.Recv()
		switch {
		case err == io.EOF:
			err = errors.New("server closed connection")
			fallthrough
		case err != nil:
			disconnected = true
			continue
		}

		c.channel.ToClients <- msg
	}

	if err != nil {
		return err
	}

	return nil
}

func (c *ChatClientService) Close() error {
	err := c.conn.CloseSend()
	if err != nil {
		return err
	}

	defer c.channel.Teardown()

	return c.grpcClient.Close()
}
