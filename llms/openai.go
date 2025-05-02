package llms

import (
	"context"

	"codeberg.org/n30w/jasima/memory"
	"github.com/pkg/errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// ChatGPTBaseURL is blank because the OpenAI client library assumes GPT on a
// blank base URL.
const ChatGPTBaseURL = ""

// openAIClient wraps the openai library. Use to create custom OpenAI
// API compatible LLM services.
type openAIClient struct {
	*llm
	client           *openai.Client
	completionParams *openai.ChatCompletionNewParams
}

// newOpenAIClient makes a new OpenAI API compatible client. It returns
// a function that accepts a model configuration for finer client details.
// An empty string baseUrl uses the baseUrl of OpenAI's ChatGPT.
func newOpenAIClient(
	apiKey string,
	baseUrl string,
) func(mc ModelConfig) *openAIClient {
	var c openai.Client

	if baseUrl == ChatGPTBaseURL {
		c = openai.NewClient(option.WithAPIKey(apiKey))
	} else {
		c = openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseUrl),
		)
	}

	return func(mc ModelConfig) *openAIClient {
		messages := make([]openai.ChatCompletionMessageParamUnion, 0)
		messages = append(messages, openai.SystemMessage(mc.Instructions))
		return &openAIClient{
			llm: &llm{
				model:          mc.Provider,
				instructions:   mc.Instructions,
				responseFormat: ResponseFormatText,
			},
			client: &c,
			completionParams: &openai.ChatCompletionNewParams{
				Seed:                openai.Int(1),
				MaxCompletionTokens: openai.Int(3000),
				Temperature:         openai.Float(mc.Temperature),
				TopP:                openai.Float(1.0),
				Messages:            messages,
				FrequencyPenalty:    openai.Float(1.1),
				PresencePenalty:     openai.Float(1.2),
				Model:               mc.Provider.String(),
			},
		}
	}
}

func (c openAIClient) Request(
	ctx context.Context,
	messages []memory.Message,
	_ string,
) (string, error) {
	switch c.responseFormat {
	case ResponseFormatJson:
		schema := openai.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:        "",
			Strict:      openai.Bool(true),
			Description: openai.String(""),
			Schema:      "",
		}
		c.completionParams.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schema},
		}
	}

	c.completionParams.Messages = c.prepare(messages)

	result, err := c.client.Chat.Completions.New(
		ctx,
		*c.completionParams,
	)
	if err != nil {
		return "", errors.Wrap(
			err,
			"openai client failed to send request to llm",
		)
	}

	return result.Choices[0].Message.Content, nil
}

func (c openAIClient) prepare(
	messages []memory.Message,
) []openai.ChatCompletionMessageParamUnion {
	contents := make([]openai.ChatCompletionMessageParamUnion, 0)

	instructions := openai.SystemMessage(c.instructions)

	contents = append(contents, instructions)

	l := len(messages)

	if l != 0 {
		for _, v := range messages {

			text := v.Text.String()

			var content openai.ChatCompletionMessageParamUnion

			content = openai.UserMessage(text)

			if v.Role.String() == "model" {
				content = openai.AssistantMessage(text)
			}

			contents = append(contents, content)
		}
	}

	return contents
}

func (c openAIClient) SetInstructions(s string) {
	c.instructions = s
}

func (c openAIClient) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

func (c openAIClient) String() string {
	return c.model.String()
}
