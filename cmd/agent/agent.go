package main

import (
	"context"

	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/memory"
)

type LLMService interface {
	// String returns the full name of the service provider and the model type.
	String() string

	// Request sends a request to the remote service. Returns a reply in the form
	// of a string. Note that rather than serializing messages into a string,
	// which would remove dependence on the `memory` package, a slice of
	// messages is passed in directly, because it allows different services to
	// adapt the messages to their different submission formats of their
	// respective APIs.
	Request(
		ctx context.Context,
		messages []memory.Message,
	) (string, error)

	// SetInstructions sets the initial instructions for the model.
	SetInstructions(s string)

	// AppendInstructions appends instructions to the initial instructions of
	// the model.
	AppendInstructions(s string)
}

// MemoryService is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type MemoryService interface {
	// Save saves a message, using its role and text. A role of `0` saves as
	// "user". A role of `1` saves as "model".
	Save(ctx context.Context, message memory.Message) error

	// Retrieve retrieves an `n` amount of messages from the storage. An `n`
	// less-than-or-equal-to zero returns all messages. Any `n` amount
	// less-than-or-equal-to the total number of memories returns `n` messages.
	// `name` is the name of the agent that inserted the messages. This is
	// just the client name.
	Retrieve(ctx context.Context, name chat.Name, n int) (
		[]memory.Message,
		error,
	)

	// Clear clears all the memory in the storage.
	Clear() error
}
