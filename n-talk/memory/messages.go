package memory

import (
	"time"
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
	InsertedBy string
	Sender     string
	Receiver   string
}

func NewMessage(role ChatRole, text string) Message {
	return Message{
		Role: role,
		Text: text,
	}
}
