package server

import (
	"fmt"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"github.com/charmbracelet/log"
)

// client represents a client connected to the server.
type client struct {
	stream pb.ChatService_ChatServer
	name   string
	model  string
	layer  int32
}

func NewClient(
	stream pb.ChatService_ChatServer,
	name, model string,
	layer int32,
) (*client, error) {
	if name == "" {
		return nil, fmt.Errorf("client name cannot be empty")
	}

	c := &client{
		stream: stream,
		name:   name,
		model:  model,
		layer:  layer,
	}

	return c, nil
}

func (c *client) Send(msg *pb.Message) error {
	return c.send(msg)
}

func (c *client) send(msg *pb.Message) error {
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

type (
	layerToNamesMap  map[int32]map[string]struct{}
	nameToClientsMap map[string]*client
)

type clientele struct {
	byName  nameToClientsMap
	byLayer layerToNamesMap
	logger  *log.Logger
}

func (ct *clientele) addByName(c *client) {
	ct.byName[c.name] = c
}

func (ct *clientele) removeByName(c *client) {
	delete(ct.byName, c.name)
}

func (ct *clientele) addByLayer(c *client) {
	_, ok := ct.byLayer[c.layer]
	if !ok {
		ct.byLayer[c.layer] = make(map[string]struct{})
	}

	ct.byLayer[c.layer][c.name] = struct{}{}
}

func (ct *clientele) removeByLayer(c *client) {
	delete(ct.byLayer[c.layer], c.name)
}

func (ct *clientele) getByName(n string) (*client, bool) {
	c, ok := ct.byName[n]
	return c, ok
}

// getByLayer receives a layer parameter to retrieve an `n` list of clients with
// that specified layer.
func (ct *clientele) getByLayer(layer int32) []*client {
	clients := make([]*client, 0)
	l := ct.byLayer[layer]

	for k := range l {
		c, ok := ct.getByName(k)
		if ok {
			clients = append(clients, c)
		}
	}

	return clients
}
