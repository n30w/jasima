package main

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/llms"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"
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

// typedRequest dispatches a JSON request to a remote LLM service. For now, the
// response is serialized as a string so that it can be sent over gRPC. Ideally,
// this should be changed so that the gRPC channel accepts type `T` from the
// request return, however this will do for now.
func typedRequest[T any](
	ctx context.Context, msg *memory.Message, c *client,
) {
	if msg.Text == "" {
		c.logger.Warn("Empty message text...")
		return
	}

	err := c.memory.Save(
		ctx,
		c.NewMessageFrom(msg.Sender, msg.Text),
	)
	if err != nil {
		c.channels.errs <- err
		return
	}

	a, err := c.memory.Retrieve(ctx, c.Name, 0)
	if err != nil {
		c.channels.errs <- errors.Wrap(
			err,
			"error getting memory messages",
		)
		return
	}

	t := utils.Timer(time.Now())

	result, err := selectRequestType[T](
		ctx,
		a,
		c,
	)
	if err != nil {
		c.channels.errs <- errors.Wrap(err, "failed response from typed LLM")
		return
	}

	c.logger.Debugf(
		"Response received from LLM, roundtrip %s",
		t().Truncate(1*time.Millisecond),
	)

	newMsg := c.NewMessageTo(c.Name, chat.Content(result))

	err = c.memory.Save(ctx, newMsg)
	if err != nil {
		c.channels.errs <- errors.Wrap(
			err,
			"failed saving new message to memory",
		)
		return
	}

	c.channels.responses <- newMsg

	c.logger.Debug("Message sent to response channel")
}

// selectRequestType returns the result of a particular request given type `T`.
// `T` enforces the JSON schema of the request's body.
func selectRequestType[T any](
	ctx context.Context,
	messages []memory.Message, c *client,
) (string, error) {
	switch c.ModelConfig.Provider {
	case llms.ProviderGoogleGemini_2_0_Flash:
		fallthrough
	case llms.ProviderGoogleGemini_2_5_Flash:
		return llms.RequestTypedGoogleGemini[T](
			ctx,
			messages,
			c.llmServices.gemini,
		)
	case llms.ProviderChatGPT:
		return llms.RequestTypedChatGPT[T](
			ctx,
			messages,
			c.llmServices.chatgpt,
		)
	default:
		c.logger.Warnf(
			"JSON schema request for %s not supported, "+
				"using default request method",
			c.ModelConfig.Provider,
		)
		return c.llm.Request(ctx, messages)
	}
}
