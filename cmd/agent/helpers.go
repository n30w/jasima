package main

import (
	"time"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
)

func (c *client) newMessage(text chat.Content) memory.Message {
	return memory.Message{
		Text:       text,
		Timestamp:  time.Now(),
		InsertedBy: c.Name,
	}
}

func (c *client) NewMessageFrom(
	sender chat.Name,
	text chat.Content,
) memory.Message {
	m := c.newMessage(text)

	m.Role = memory.UserRole
	m.Sender = sender
	m.Receiver = c.Name
	m.Layer = c.Layer

	return m
}

func (c *client) NewMessageTo(
	recipient chat.Name,
	text chat.Content,
) memory.Message {
	m := c.newMessage(text)

	m.Role = memory.ModelRole
	m.Receiver = recipient
	m.Sender = c.Name
	m.Layer = c.Layer

	return m
}

func (c *client) sendMessage(msg memory.Message) error {
	m := chat.NewPbMessage(c.Name, c.Peers[0], msg.Text, c.Layer)

	err := c.mc.Send(m)
	if err != nil {
		return err
	}

	return nil
}

// initConnection runs to establish an initial connection to the server.
func (c *client) initConnection() error {
	content := chat.Content(c.llm.String())

	msg := c.NewMessageTo(c.Peers[0], content)

	c.channels.responses <- msg

	c.logger.Debugf(
		"Established connection to the server @ %s",
		c.NetworkConfig.Router,
	)

	return nil
}
