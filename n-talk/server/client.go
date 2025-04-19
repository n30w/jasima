package server

import (
	"fmt"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/charmbracelet/log"
)

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

func (c *client) Send(msg *memory.Message) error {
	pbMsg := chat.NewPbMessage(msg.Sender, msg.Receiver, msg.Text, msg.Layer)

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
