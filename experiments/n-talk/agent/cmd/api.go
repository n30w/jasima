package main

import (
	"context"

	"google.golang.org/genai"
)

type client struct {
	model       string
	genaiClient *genai.Client
}

func NewClient(ctx context.Context, apiKey string, model string) (*client, error) {
	g, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	c := &client{
		model:       model,
		genaiClient: g,
	}

	return c, nil
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {
	result, err := c.genaiClient.Models.GenerateContent(ctx, c.model, genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}
	res, err := result.Text()
	if err != nil {
		return "", err
	}

	return res, nil
}
