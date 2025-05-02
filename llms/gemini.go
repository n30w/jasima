package llms

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"

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
			model:          mc.Provider,
			responseFormat: ResponseFormatText,
		},
		genaiClient: g,
		genaiConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(mc.Temperature)),
			MaxOutputTokens: 8192,
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
		// Jack it up because we can.
		c.genaiConfig.MaxOutputTokens = 32767
	}

	return c, nil
}

func (c GoogleGemini) Request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	switch c.responseFormat {
	case ResponseFormatJson:
		p, err := makeGoogleGeminiSchemaProperties("")
		if err != nil {
			return "", err
		}
		c.genaiConfig.ResponseMIMEType = "application/json"
		c.genaiConfig.ResponseSchema = &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: p,
			},
		}
	}

	contents := c.prepare(messages)

	result, err := c.genaiClient.Models.GenerateContent(
		ctx,
		c.model.String(),
		contents,
		c.genaiConfig,
	)
	if err != nil {
		return "", errors.Wrap(err, "gemini client failed to make request")
	}

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

			content := genai.NewContentFromText(
				v.Text.String(),
				genai.RoleUser,
			)

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

func makeGoogleGeminiSchemaProperties(d any) (
	map[string]*genai.Schema,
	error,
) {
	m := make(map[string]*genai.Schema)

	v := reflect.ValueOf(d)
	t := reflect.TypeOf(d)

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%v is not a struct", d)
	}

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		switch value.Kind() {
		case reflect.String:
			m[field.Name] = &genai.Schema{
				Type: genai.TypeString,
			}
		}
	}

	return m, nil
}
