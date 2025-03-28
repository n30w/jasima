package main

import (
	"fmt"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
)

type client struct {
	stream pb.ChatService_ChatServer
	name   string
	model  string
	layer  int
}

func NewClient(stream pb.ChatService_ChatServer, name, model string) (*client, error) {
	if name == "" {
		return nil, fmt.Errorf("client name cannot be empty")
	}

	c := &client{
		stream: stream,
		name:   name,
		model:  model,
	}

	return c, nil
}

func (c *client) Send(msg *pb.Message) error {
	err := c.stream.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to: %s: %v", msg.Receiver, err)
	}

	return nil
}

func (c client) String() string {
	return fmt.Sprintf("%s [%s]", c.name, c.model)
}
