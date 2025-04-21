package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"google.golang.org/genai"
)

type GoogleGemini struct {
	*llm
	genaiClient *genai.Client
	genaiConfig *genai.GenerateContentConfig
}

func (c GoogleGemini) SetInstructions(s string) {
	c.instructions = s
}

func (c GoogleGemini) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

func NewGoogleGemini(
	ctx context.Context,
	apiKey string,
	mc ModelConfig,
) (*GoogleGemini, error) {
	g, err := genai.NewClient(
		ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		},
	)
	if err != nil {
		return nil, err
	}

	c := &GoogleGemini{
		llm: &llm{
			model: ProviderGoogleGemini,
		},
		genaiClient: g,
		genaiConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(mc.Temperature),
			MaxOutputTokens: genai.Ptr(int64(10000)),
		},
	}

	if mc.Instructions != "" {
		c.genaiConfig.SystemInstruction = genai.NewModelContentFromText(
			mc.Instructions,
		)
	}

	return c, nil
}

func (c GoogleGemini) Request(
	ctx context.Context,
	messages []memory.Message,
	prompt string,
) (string, error) {
	contents := c.prepare(messages)
	contents = append(contents, genai.NewUserContentFromText(prompt))

	result, err := c.genaiClient.Models.GenerateContent(
		ctx,
		c.model.String(),
		contents,
		c.genaiConfig,
	)
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

// prepare adheres memories to the `genai` library `content` type.
func (c GoogleGemini) prepare(messages []memory.Message) []*genai.Content {
	l := len(messages)

	contents := make([]*genai.Content, l)

	// If the memory isn't empty, append the memory to the content
	// for the request.
	if l != 0 {
		for i, v := range messages {

			var content *genai.Content

			content = genai.NewUserContentFromText(v.Text.String())

			if v.Role.String() == "model" {
				content = genai.NewModelContentFromText(v.Text.String())
			}

			contents[i] = content
		}
	}

	return contents
}

func (c GoogleGemini) String() string {
	return fmt.Sprintf("Google Gemini %s", c.model)
}
