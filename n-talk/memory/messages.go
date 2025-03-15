package memory

import (
	"context"
	"errors"
	"sync"
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
	Sender    string
	Receiver  string
}

func NewMessage(role ChatRole, text string) Message {
	return Message{
		Role: role,
		Text: text,
	}
}

type InMemoryStore struct {
	messages []Message

	// total is the number of memories in the store
	total int

	mu sync.Mutex
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make([]Message, 0),
		total:    0,
	}
}

func (in *InMemoryStore) Save(_ context.Context, role ChatRole, text string) error {

	t := time.Now()

	msg := NewMessage(role, text)

	msg.Timestamp = t

	in.mu.Lock()
	in.messages = append(in.messages, msg)
	in.mu.Unlock()

	in.total = len(in.messages)

	return nil
}

func (in *InMemoryStore) Retrieve(_ context.Context, n int) ([]Message, error) {

	in.mu.Lock()
	defer in.mu.Unlock()

	if n > in.total {
		return nil, errors.New("too many entries requested")
	}

	// Retrieve all memories
	if n <= 0 {
		return in.messages, nil
	}

	messages := make([]Message, n)

	for i := in.total - n; i < in.total; i++ {
		messages = append(messages, in.messages[i])
	}

	return messages, nil
}
