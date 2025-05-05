package network

import (
	"fmt"
	"io"
	"net"
	"sync"

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

type GRPCServer struct {
	chat.UnimplementedChatServiceServer
	clients *clientele
	mu      sync.Mutex
	Name    chat.Name
	logger  *log.Logger
	Channel channels

	// listening determines whether the server will operate on messages,
	// whether it be through routing, saving, etc.
	Listening bool
}

func NewGRPCServer(logger *log.Logger, name string) *GRPCServer {
	clients := &clientele{
		byNameMap:  make(nameToClientsMap),
		byLayerMap: make(layerToNamesMap),
	}
	chs := channels{
		ToClients: make(chan *chat.Message),
		ToServer:  make(memory.MessageChannel),
	}
	return &GRPCServer{
		Name:      chat.Name(name),
		Listening: true,
		Channel:   chs,
		clients:   clients,
		logger:    logger,
	}
}

func (s *GRPCServer) ListenAndServe(
	protocol, port string,
	errs chan<- error,
) {
	p := makePortString(port)

	lis, err := net.Listen(protocol, p)
	if err != nil {
		errs <- err
		return
	}

	grpcServer := grpc.NewServer()

	chat.RegisterChatServiceServer(grpcServer, s)

	s.logger.Infof("Starting gRPC service on %s%s", protocol, p)

	err = grpcServer.Serve(lis)
	if err != nil {
		errs <- err
		return
	}
}

// Chat is called by the `GRPCClient`. The lifetime of this function is for as
// long as the GRPCClient using this function is connected.
func (s *GRPCServer) Chat(stream chat.ChatService_ChatServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return err
	}

	c, err := s.initClient(stream, firstMsg)
	if err != nil {
		return err
	}

	// Enter an infinite listening session when the GRPCClient is connected.
	// Each GRPCClient receives their own context. `listen` is a blocking call.

	err = s.listen(c)

	s.removeClient(c)

	if err == io.EOF {
		s.logger.Info("Client disconnected", "GRPCClient", c.Name)
	} else if err != nil {
		return errors.Wrap(err, "unexpected GRPCClient disconnection")
	}

	return nil
}

// listen is called when a GRPCClient connection with `Chat` has already been
// established. It disconnects clients when they error or when they disconnect
// from the server. It also calls `routeMessage` when a message is received
// from the connected GRPCClient.
func (s *GRPCServer) listen(
	c *GRPCClient,
) error {
	var (
		err          error
		msg          *chat.Message
		disconnected bool
	)

	for !disconnected {

		// Wait for a message to come in from the GRPCClient. This is a blocking call.

		msg, err = c.stream.Recv()
		if err != nil {
			disconnected = true
			continue
		}

		select {
		case s.Channel.ToClients <- msg:
		default:
		}

		c.mu.Lock()

		for ch := range c.channels {
			select {
			case ch <- msg:
			default:
			}
		}

		c.mu.Unlock()

	}

	if err != nil {
		return err
	}

	return nil
}

// Broadcast forwards a message `msg` to all clients on a layer, excluding the
// sender. If a message has peers to send to, the message will not be broadcast
// across the entire layer but only broadcasted to the peer in question.
func (s *GRPCServer) Broadcast(msg *memory.Message) error {
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

// forward forwards a message `msg` to a GRPCClient. The GRPCClient should exist in
// the list of clients maintaining an active connection. routeMessage returns
// an error if the GRPCClient does not exist.
func (s *GRPCServer) forward(msg *memory.Message) error {
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

func (s *GRPCServer) TotalClients() int {
	return s.clients.total
}

// initClient initializes a GRPCClient connection and adds the GRPCClient to the
// list of clients currently maintaining a connection.
func (s *GRPCServer) initClient(
	stream chat.ChatService_ChatServer,
	msg *chat.Message,
) (*GRPCClient, error) {
	c, err := newClient(stream, msg.Sender, msg.Content, msg.Layer)
	if err != nil {
		return nil, err
	}

	s.addClient(c)

	s.logger.Info(
		"Client connected",
		"GRPCClient",
		c.String(),
		"Layer",
		c.layer,
	)

	return c, nil
}

func (s *GRPCServer) AddClient(c *GRPCClient) {
	s.addClient(c)
}

func (s *GRPCServer) RemoveClient(c *GRPCClient) {
	s.removeClient(c)
}

func (s *GRPCServer) GetClientsByLayer(layer chat.Layer) []*GRPCClient {
	return s.getClientsByLayer(layer)
}

func (s *GRPCServer) GetClientByName(name chat.Name) (*GRPCClient, error) {
	return s.getClientByName(name)
}

// addClient adds a GRPCClient to the list of clients that maintain an active
// connection to the server.
func (s *GRPCServer) addClient(client *GRPCClient) {
	// Add the GRPCClient to the list of current clients. Multiple connections may
	// happen all at once, so we need to lock and unlock the mutex to avoid
	// race conditions.

	s.mu.Lock()
	s.clients.addByName(client)
	s.clients.addByLayer(client)
	s.clients.total++
	s.mu.Unlock()
}

// removeClient removes a GRPCClient from the list of clients that maintain an
// active connection to the server.
func (s *GRPCServer) removeClient(client *GRPCClient) {
	s.mu.Lock()
	s.clients.removeByName(client)
	s.clients.removeByLayer(client)
	s.clients.total--
	s.mu.Unlock()
}

// getClientsByLayer retrieves all the clients of a Layer and returns them
// in an array of pointers to those clients.
func (s *GRPCServer) getClientsByLayer(layer chat.Layer) []*GRPCClient {
	var c []*GRPCClient

	s.mu.Lock()
	c = s.clients.byLayer(layer)
	s.mu.Unlock()

	return c
}

func (s *GRPCServer) getClientByName(name chat.Name) (*GRPCClient, error) {
	var c *GRPCClient
	var ok bool

	c, ok = s.clients.byName(name)

	if !ok {
		return nil, fmt.Errorf("GRPCClient with name: '%s' not found", name)
	}

	return c, nil
}

// GRPCClient represents a GRPCClient connected to the server.
type GRPCClient struct {
	Name     chat.Name
	stream   chat.ChatService_ChatServer
	model    string
	layer    chat.Layer
	channels map[chan *chat.Message]struct{}
	mu       sync.Mutex
}

func newClient(
	stream chat.ChatService_ChatServer,
	name, model string,
	l int32,
) (*GRPCClient, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	c := &GRPCClient{
		stream:   stream,
		Name:     chat.Name(name),
		model:    model,
		layer:    chat.Layer(l),
		mu:       sync.Mutex{},
		channels: make(map[chan *chat.Message]struct{}),
	}

	return c, nil
}

func (c *GRPCClient) SendWithChannel(
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

func (c *GRPCClient) Send(msg *memory.Message, command ...agent.Command) error {
	pbMsg := chat.NewPbMessage(
		msg.Sender, msg.Receiver, msg.Text, msg.Layer,
		command...,
	)

	return c.send(pbMsg)
}

func (c *GRPCClient) send(msg *chat.Message) error {
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

func (c *GRPCClient) String() string {
	return fmt.Sprintf("%s<%s>", c.Name, c.model)
}

type (
	namesMap         map[chat.Name]struct{}
	layerToNamesMap  map[chat.Layer]namesMap
	nameToClientsMap map[chat.Name]*GRPCClient
)

type clientele struct {
	byNameMap  nameToClientsMap
	byLayerMap layerToNamesMap
	total      int
	logger     *log.Logger
}

func (ct *clientele) addByName(c *GRPCClient) {
	ct.byNameMap[c.Name] = c
}

func (ct *clientele) removeByName(c *GRPCClient) {
	delete(ct.byNameMap, c.Name)
}

func (ct *clientele) addByLayer(c *GRPCClient) {
	_, ok := ct.byLayerMap[c.layer]
	if !ok {
		ct.byLayerMap[c.layer] = make(namesMap)
	}

	ct.byLayerMap[c.layer][c.Name] = struct{}{}
}

func (ct *clientele) removeByLayer(c *GRPCClient) {
	delete(ct.byLayerMap[c.layer], c.Name)
}

func (ct *clientele) byName(n chat.Name) (*GRPCClient, bool) {
	c, ok := ct.byNameMap[n]
	return c, ok
}

// byLayer receives a Layer parameter to retrieve an `n` list of clients with
// that specified Layer.
func (ct *clientele) byLayer(layer chat.Layer) []*GRPCClient {
	clients := make([]*GRPCClient, 0)
	l := ct.byLayerMap[layer]

	for k := range l {
		c, ok := ct.byName(k)
		if ok {
			clients = append(clients, c)
		}
	}

	return clients
}
