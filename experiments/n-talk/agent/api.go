package main

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// client wraps a generative AI client.
type client struct {
	memory      Memory
	model       string
	genaiClient *genai.Client
	genaiConfig *genai.GenerateContentConfig
}

func NewClient(ctx context.Context, apiKey string, model string, mem Memory) (*client, error) {
	g, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	c := &client{
		memory:      mem,
		model:       model,
		genaiClient: g,
		genaiConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(1.5),
			MaxOutputTokens: genai.Ptr(int64(2000)),
			// SystemInstruction: genai.NewModelContentFromText("You are a cat named Neko"),
		},
	}

	return c, nil
}

func (c *client) Request(ctx context.Context, prompt string) (string, error) {

	contents := c.getContent()
	contents = append(contents, genai.NewUserContentFromText(prompt))

	result, err := c.genaiClient.Models.GenerateContent(ctx, c.model, contents, c.genaiConfig)
	if err != nil {
		return "", err
	}

	// res, err := json.MarshalIndent(*result, "", "  ")
	// if err != nil {
	// 	return "", err
	// }

	// return string(res), nil

	res, err := result.Text()
	if err != nil {
		return "", err
	}

	return res, nil
}

func (c *client) RequestStream(ctx context.Context, prompt string) error {

	contents := c.getContent()
	contents = append(contents, genai.NewUserContentFromText(prompt))

	// Retrieved from:
	// https://github.com/googleapis/go-genai/blob/main/samples/generate_text_stream.go
	for result, err := range c.genaiClient.Models.GenerateContentStream(ctx, c.model, contents, c.genaiConfig) {
		if err != nil {
			return err
		}
		fmt.Print(result.Candidates[0].Content.Parts[0].Text)
	}

	return nil
}

// getContent makes adheres memories to the `genai` library `content` type.
func (c *client) getContent() []*genai.Content {
	contents := make([]*genai.Content, 0)

	// If the memory isn't empty, append the memory to the content
	// for the request.
	if !c.memory.IsEmpty() {
		for _, v := range c.memory.All() {

			var content *genai.Content

			content = genai.NewUserContentFromText(v.Text)

			if v.Role == "model" {
				content = genai.NewModelContentFromText(v.Text)
			}

			contents = append(contents, content)
		}
	}

	return contents
}
