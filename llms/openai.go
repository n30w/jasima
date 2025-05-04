package llms

import (
	"context"

	"codeberg.org/n30w/jasima/memory"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/pkg/errors"
)

// ChatGPTBaseURL is blank because the OpenAI client library assumes GPT on a
// blank base URL.
const ChatGPTBaseURL = ""

// openAIClient wraps the openai library. Use to create custom OpenAI
// API compatible LLM services.
type openAIClient struct {
	*llm
	client *openai.Client
}

// newOpenAIClient makes a new OpenAI API compatible client. It returns
// a function that accepts a model configuration for finer client details.
// An empty string baseUrl uses the baseUrl of OpenAI's ChatGPT.
func newOpenAIClient(
	apiKey string,
	baseUrl string,
	logger *log.Logger,
) func(mc ModelConfig) (*openAIClient, error) {
	var c openai.Client

	if baseUrl == ChatGPTBaseURL {
		c = openai.NewClient(option.WithAPIKey(apiKey))
	} else {
		c = openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseUrl),
		)
	}

	return func(mc ModelConfig) (*openAIClient, error) {
		m := make([]openai.ChatCompletionMessageParamUnion, 0)
		m = append(m, openai.SystemMessage(mc.Instructions))

		l, err := newLLM(mc, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new openAI client")
		}

		return &openAIClient{
			llm:    l,
			client: &c,
		}, nil
	}
}

func (c openAIClient) buildRequestParams(rc *RequestConfig) *openai.
	ChatCompletionNewParams {
	params := &openai.ChatCompletionNewParams{
		Seed:                openai.Int(c.defaultConfig.Seed),
		MaxCompletionTokens: openai.Int(c.defaultConfig.MaxTokens),
		Temperature:         openai.Float(c.defaultConfig.Temperature),
		PresencePenalty:     openai.Float(c.defaultConfig.PresencePenalty),
		FrequencyPenalty:    openai.Float(c.defaultConfig.FrequencyPenalty),
		Model:               c.model.String(),
	}

	// If a config is provided, use it.

	if rc != nil {
		params = &openai.ChatCompletionNewParams{
			Seed:                openai.Int(rc.Seed),
			MaxCompletionTokens: openai.Int(rc.MaxTokens),
			Temperature:         openai.Float(rc.Temperature),
			PresencePenalty:     openai.Float(rc.PresencePenalty),
			FrequencyPenalty:    openai.Float(rc.FrequencyPenalty),
			Model:               c.model.String(),
		}
	}

	return params
}

func (c openAIClient) request(
	ctx context.Context,
	messages []memory.Message,
	opts ...func(cfg *openai.ChatCompletionNewParams),
) (string, error) {
	p := c.buildRequestParams(nil)

	p.Messages = c.prepare(messages)

	for _, opt := range opts {
		opt(p)
	}

	result, err := c.client.Chat.Completions.New(
		ctx,
		*p,
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

	if len(messages) != 0 {
		for _, v := range messages {

			text := v.Text.String()

			var content openai.ChatCompletionMessageParamUnion

			content = openai.UserMessage(text)

			if v.Role == memory.ModelRole {
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

func newOpenAIResponseSchema(schema any) openai.
	ResponseFormatJSONSchemaJSONSchemaParam {
	return openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        agentResponseName,
		Strict:      openai.Bool(true),
		Description: openai.String(agentResponseDescription),
		Schema:      schema,
	}
}
