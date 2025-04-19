package server

import (
	"fmt"

	"codeberg.org/n30w/jasima/n-talk/internal/commands"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/charmbracelet/log"
)

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

// client represents a client connected to the server.
type client struct {
	stream chat.ChatService_ChatServer
	name   chat.Name
	model  string
	layer  chat.Layer
}

func newClient(
	stream chat.ChatService_ChatServer,
	name, model string,
	l int32,
) (*client, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	c := &client{
		stream: stream,
		name:   chat.Name(name),
		model:  model,
		layer:  chat.Layer(l),
	}

	return c, nil
}

func (c *client) Send(msg *memory.Message, command ...commands.Command) error {
	pbMsg := chat.NewPbMessage(
		msg.Sender, msg.Receiver, msg.Text, msg.Layer,
		command...,
	)

	return c.send(pbMsg)
}

func (c *client) send(msg *chat.Message) error {
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

func (c *client) String() string {
	return fmt.Sprintf("%s<%s>", c.name, c.model)
}

type Names map[chat.Name]struct{}

type Clients []*client

type (
	layerToNamesMap  map[chat.Layer]Names
	nameToClientsMap map[chat.Name]*client
)

type clientele struct {
	byNameMap  nameToClientsMap
	byLayerMap layerToNamesMap
	logger     *log.Logger
}

func (ct *clientele) addByName(c *client) {
	ct.byNameMap[c.name] = c
}

func (ct *clientele) removeByName(c *client) {
	delete(ct.byNameMap, c.name)
}

func (ct *clientele) addByLayer(c *client) {
	_, ok := ct.byLayerMap[c.layer]
	if !ok {
		ct.byLayerMap[c.layer] = make(Names)
	}

	ct.byLayerMap[c.layer][c.name] = struct{}{}
}

func (ct *clientele) removeByLayer(c *client) {
	delete(ct.byLayerMap[c.layer], c.name)
}

func (ct *clientele) byName(n chat.Name) (*client, bool) {
	c, ok := ct.byNameMap[n]
	return c, ok
}

// byLayer receives a Layer parameter to retrieve an `n` list of clients with
// that specified Layer.
func (ct *clientele) byLayer(layer chat.Layer) Clients {
	clients := make([]*client, 0)
	l := ct.byLayerMap[layer]

	for k := range l {
		c, ok := ct.byName(k)
		if ok {
			clients = append(clients, c)
		}
	}

	return clients
}
