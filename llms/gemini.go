package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/memory"

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
			model: mc.Provider,
		},
		genaiClient: g,
		genaiConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(mc.Temperature)),
			MaxOutputTokens: 10000,
		},
	}

	if mc.Instructions != "" {
		c.genaiConfig.SystemInstruction = genai.NewContentFromText(
			mc.Instructions, genai.RoleModel,
		)
	}

	if mc.Provider == ProviderGoogleGemini_2_5_Flash {
		// Gemini 2.5 lets you toggle whether thinking is on or off, via
		// the `ThinkingBudget` parameter. Setting it to 0 makes it not
		// think. Gemini 2.0 does not provide this capability.
		c.genaiConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr(int32(0)),
		}
	}

	return c, nil
}

func (c GoogleGemini) Request(
	ctx context.Context,
	messages []memory.Message,
	prompt string,
) (string, error) {
	contents := c.prepare(messages)
	contents = append(
		contents,
		genai.NewContentFromText(prompt, genai.RoleUser),
	)

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

	return result.Text(), nil
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

			content = genai.NewContentFromText(v.Text.String(), genai.RoleUser)

			if v.Role.String() == "model" {
				content = genai.NewContentFromText(
					v.Text.String(),
					genai.RoleModel,
				)
			}

			contents[i] = content
		}
	}

	return contents
}

func (c GoogleGemini) String() string {
	return fmt.Sprintf("Google Gemini %s", c.model)
}
