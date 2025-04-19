package memory

import (
	"time"

	"codeberg.org/n30w/jasima/n-talk/internal/chat"
	"codeberg.org/n30w/jasima/n-talk/internal/commands"
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
	Text      string
	Timestamp time.Time
	Id        int64

	// InsertedBy represents the agent who inserted the message. This is
	// used to identify and query for a specific user's messages, since only
	// one SQL table is used for all messages.
	InsertedBy chat.Name
	Sender     chat.Name
	Receiver   chat.Name
	Layer      chat.Layer
	Command    commands.Command
}

func NewMessage(role ChatRole, text string) Message {
	return Message{
		Role: role,
		Text: text,
	}
}

func NewChatMessage(
	sender, receiver string, text string, layer int32,
	command ...int32,
) *Message {
	msg := Message{
		Sender:   chat.Name(sender),
		Receiver: chat.Name(receiver),
		Text:     text,
		Layer:    chat.Layer(layer),
	}

	if len(command) > 0 {
		msg.Command = commands.Command(command[0])
	}

	return &msg
}
