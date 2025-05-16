package main

import (
	"context"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/llms"
	"codeberg.org/n30w/jasima/pkg/memory"
)

// llmServices defines various LLM clients the client may use to make requests.
type llmServices struct {
	gemini  *llms.GoogleGemini
	chatgpt *llms.OpenAIChatGPT
	ollama  *llms.Ollama
}

type llmService interface {
	// String returns the full name of the service provider and the model type.
	String() string

	// Request sends a request to the remote service. Returns a reply in the form
	// of a string. Note that rather than serializing messages into a string,
	// which would remove dependence on the `memory` package, a slice of
	// messages is passed in directly, because it allows different services to
	// adapt the messages to their different submission formats of their
	// respective APIs.
	Request(ctx context.Context, messages []memory.Message) (string, error)

	// SetInstructions sets the initial instructions for the model.
	SetInstructions(s string)

	// AppendInstructions appends instructions to the initial instructions of
	// the model.
	AppendInstructions(s string)
}

// memoryServices defines different memory repositories the agent may use to
// throughout its lifetime.
type memoryServices struct {
	// stm, or short-term memory, is the working chat memory, which saves
	// messages during inter-agent exchanges. It may be cleared after
	// exchanges are done.
	stm memoryService

	// ltm, or long-term memory, stores information long-term. This information
	// is collected from previous exchanges. The information is often a
	// summary of old exchanges.
	ltm memoryService
}

// memoryService is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type memoryService interface {
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

type messageService[T any] interface {
	// Receive blocks until a message is received. It then returns the message
	// and an error, if there is one.
	Receive() error

	// Send sends a message of type `T`.
	Send(*T) error

	// Close closes the connection to the server.
	Close() error
}
