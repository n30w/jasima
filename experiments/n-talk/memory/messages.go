package memory

import (
	"errors"
	"sync"
	"time"
)

type Message struct {
	Role      string
	Text      string
	Timestamp time.Time
	Id        int64
	Sender    string
	Receiver  string
}

func NewMessage(role string, text string) Message {
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

func (in *InMemoryStore) Save(role int, text string) error {

	t := time.Now()

	var msg Message

	switch role {
	case 0:
		msg = NewMessage("user", text)
	case 1:
		msg = NewMessage("model", text)
	default:
		msg = NewMessage("model", text)
	}

	msg.Timestamp = t

	in.mu.Lock()
	in.messages = append(in.messages, msg)
	in.mu.Unlock()

	in.total = len(in.messages)

	return nil
}

func (in *InMemoryStore) Retrieve(n int) ([]Message, error) {

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
