package main

import (
	"context"
)

// client wraps a generative AI client.
type client struct {
	memory Memory
	llm    LLMService
}

func NewClient(ctx context.Context, llm LLMService, memory Memory) (*client, error) {
	c := &client{
		memory: memory,
		llm:    llm,
	}

	return c, nil
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {
	a := c.memory.All()

	result, err := c.llm.Request(ctx, a, prompt)
	if err != nil {
		return "", err
	}

	return result, nil
}
