package memory

import (
	"errors"
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

func NewMessage(role string, text string) *Message {
	return &Message{
		Role: role,
		Text: text,
	}
}

type InMemoryStore struct {
	Memories []*Message

	// total is the number of memories in the store
	total int
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Memories: make([]*Message, 0),
		total:    0,
	}
}

func (in *InMemoryStore) All() []*Message {
	return in.Memories
}

func (in *InMemoryStore) Save(s string) error {
	in.Memories = append(in.Memories, &Message{Role: "user", Text: s})
	in.total = len(in.Memories)
	return nil
}

func (in *InMemoryStore) Retrieve(e int) error {

	if e > in.total {
		return errors.New("too many entries requested")
	}

	// Retrieve all memories
	if e <= 0 {

	}

	return nil
}

func (in *InMemoryStore) IsEmpty() bool {
	return in.total <= 0
}
