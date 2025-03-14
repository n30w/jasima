package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/n-talk/memory"
	"google.golang.org/genai"
)

type GoogleGemini struct {
	*llm
	genaiClient *genai.Client
	genaiConfig *genai.GenerateContentConfig
}

func NewGoogleGemini(ctx context.Context, apiKey string, model string) (*GoogleGemini, error) {
	g, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	systemInstruction := "You are in conversation with another large language model. This is a natural conversation. Don't talk in bullet points. Don't talk like an LLM. Length of text is up to your discretion. Don't be too agreeable, be reasonable. Your conversational exchange does not need to be back and forth. You can let the other speaker know that you'll listen to what they'll have to say."

	c := &GoogleGemini{
		llm: &llm{
			model: model,
		},
		genaiClient: g,
		genaiConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(1.65),
			MaxOutputTokens: genai.Ptr(int64(2000)),
		},
	}

	if systemInstruction != "" {
		c.genaiConfig.SystemInstruction = genai.NewModelContentFromText(systemInstruction)
	}

	return c, nil
}

func (c *GoogleGemini) Request(ctx context.Context, messages []memory.Message, prompt string) (string, error) {

	contents := c.prepare(messages)
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

// getContent makes adheres memories to the `genai` library `content` type.
func (c *GoogleGemini) prepare(messages []memory.Message) []*genai.Content {
	l := len(messages)

	contents := make([]*genai.Content, l)

	// If the memory isn't empty, append the memory to the content
	// for the request.
	if l != 0 {
		for i, v := range messages {

			var content *genai.Content

			content = genai.NewUserContentFromText(v.Text)

			if v.Role.String() == "model" {
				content = genai.NewModelContentFromText(v.Text)
			}

			contents[i] = content
		}
	}

	return contents
}

func (c GoogleGemini) String() string {
	return fmt.Sprintf("Google Gemini %s", c.model)
}
