package main

import (
	"context"

	"codeberg.org/n30w/jasima/n-talk/memory"
)

type LLMService interface {
	// Stringer interface.
	String() string

	// Sends a request to the remote service. Returns a reply in the form
	// of a string.
	Request(ctx context.Context, messages []memory.Message, prompt string) (string, error)
}

// MemoryService is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type MemoryService interface {
	// Save saves a message, using its role and text. A role of `0` saves as
	// "user". A role of `1` saves as "model".
	Save(role memory.ChatRole, text string) error

	// Retrieve retrieves an `n` amount of messages from the storage. An `n`
	// less-than-or-equal-to zero returns all messages. Any `n` amount
	// less-than-or-equal-to the total number of memories returns `n` messages.
	Retrieve(n int) ([]memory.Message, error)
}

type ConnectionService interface {
	Send()
	Receive()
	Open()
}
