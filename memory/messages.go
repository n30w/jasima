package memory

import (
	"time"

	"codeberg.org/n30w/jasima/chat"
	"github.com/nats-io/nats-server/v2/server"
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
	Role      ChatRole
	Text      chat.Content
	Timestamp time.Time
	Id        int64

	// InsertedBy represents the agent who inserted the message. This is
	// used to identify and query for a specific user's messages, since only
	// one SQL table is used for all messages.
	InsertedBy chat.Name
	Sender     chat.Name
	Receiver   chat.Name
	Layer      chat.Layer
	Command    server.Command
}

func NewMessage(role ChatRole, text string) Message {
	return Message{
		Role: role,
		Text: chat.Content(text),
	}
}

func NewChatMessage(
	sender, receiver string, text string, layer int32,
	command ...int32,
) *Message {
	msg := Message{
		Sender:   chat.Name(sender),
		Receiver: chat.Name(receiver),
		Text:     chat.Content(text),
		Layer:    chat.Layer(layer),
	}

	if len(command) > 0 {
		msg.Command = server.Command(command[0])
	}

	return &msg
}

type MessageChannel chan Message
