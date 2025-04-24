package memory

import (
	"time"

	"codeberg.org/n30w/jasima/agent"

	"codeberg.org/n30w/jasima/chat"
)

type ChatRole uint8

const (
	UserRole ChatRole = iota
	ModelRole
)

func (c ChatRole) String() string {
	s := "user"

	switch c {
	case 0:
		s = "user"
	case 1:
		s = "model"
	}

	return s
}

type Message struct {
	Role      ChatRole     `json:"role,omitempty"`
	Text      chat.Content `json:"text,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Id        int64        `json:"id,omitempty"`

	// InsertedBy represents the agent who inserted the message. This is
	// used to identify and query for a specific user's messages, since only
	// one SQL table is used for all messages.
	InsertedBy chat.Name     `json:"insertedBy,omitempty"`
	Sender     chat.Name     `json:"sender,omitempty"`
	Receiver   chat.Name     `json:"receiver,omitempty"`
	Layer      chat.Layer    `json:"layer,omitempty"`
	Command    agent.Command `json:"command"`
}

func NewMessage(role ChatRole, text string) Message {
	return Message{
		Timestamp: time.Now(),
		Role:      role,
		Text:      chat.Content(text),
	}
}

func NewChatMessage(
	sender, receiver string, text string, layer int32,
	command ...int32,
) *Message {
	msg := Message{
		Timestamp: time.Now(),
		Sender:    chat.Name(sender),
		Receiver:  chat.Name(receiver),
		Text:      chat.Content(text),
		Layer:     chat.Layer(layer),
	}

	if len(command) > 0 {
		msg.Command = agent.Command(command[0])
	}

	return &msg
}

type MessageChannel chan Message
