package main

import (
	"context"

	"codeberg.org/n30w/jasima/n-talk/memory"
)

type LLMService interface {
	// Sends a request to the remote service. Returns a reply in the form
	// of a string.
	Request(ctx context.Context, messages []*memory.Message, prompt string) (string, error)
}
