package llms

import (
	"context"
	"time"

	"codeberg.org/n30w/jasima/pkg/memory"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/pkg/errors"
)

// defaultChatGPTUrl is blank because the OpenAI client library assumes GPT on a
// blank base URL.
const defaultChatGPTUrl = ""

// openAIClient wraps the openai library. Use to create custom OpenAI
// API compatible LLM services.
type openAIClient struct {
	*llmBase
	chatService llmRequester[openai.ChatCompletionNewParams]
	client      *openai.Client
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

	if baseUrl == defaultChatGPTUrl {
		c = openai.NewClient(option.WithAPIKey(apiKey))
	} else {
		c = openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseUrl),
		)
	}

	return func(mc ModelConfig) (*openAIClient, error) {
		l, err := newLLM[openai.ChatCompletionNewParams](mc, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new openAI client")
		}

		return &openAIClient{
			llm:    l,
			client: &c,
		}, nil
	}
}

func makeOpenAIChatParams(rc *RequestConfig, d *RequestConfig) *openai.ChatCompletionNewParams {
	p := &openai.ChatCompletionNewParams{
		Seed:                openai.Int(d.Seed),
		MaxCompletionTokens: openai.Int(d.MaxTokens),
		Temperature:         openai.Float(d.Temperature),
		PresencePenalty:     openai.Float(d.PresencePenalty),
		FrequencyPenalty:    openai.Float(d.FrequencyPenalty),
	}

	// If a requestTypedConfig is provided, use it.

	if rc != nil {
		p = &openai.ChatCompletionNewParams{
			Seed:                openai.Int(rc.Seed),
			MaxCompletionTokens: openai.Int(rc.MaxTokens),
			Temperature:         openai.Float(rc.Temperature),
			PresencePenalty:     openai.Float(rc.PresencePenalty),
			FrequencyPenalty:    openai.Float(rc.FrequencyPenalty),
		}
	}

	return p
}

// withOpenAIModel sets the model name on the OpenAI chat params.
func withOpenAIModel(model string) func(*openai.ChatCompletionNewParams) error {
	return func(p *openai.ChatCompletionNewParams) error {
		p.Model = model
		return nil
	}
}

// withOpenAIRequestOptions applies request configuration values (temperature, penalties, etc.)
// to the OpenAI chat params, using rc if non-nil or falling back to defaultConfig.
func withOpenAIRequestOptions(
	rc *RequestConfig,
	defaultConfig *RequestConfig,
) func(*openai.ChatCompletionNewParams) error {
	return func(p *openai.ChatCompletionNewParams) error {
		base := makeOpenAIChatParams(defaultConfig, defaultConfig)
		override := makeOpenAIChatParams(rc, defaultConfig)
		// copy settings from override if rc is non-nil, else use base
		if rc != nil {
			p.Temperature = override.Temperature
			p.PresencePenalty = override.PresencePenalty
			p.FrequencyPenalty = override.FrequencyPenalty
			p.Seed = override.Seed
			p.MaxCompletionTokens = override.MaxCompletionTokens
			// copy other fields as needed
		} else {
			p.Temperature = base.Temperature
			p.PresencePenalty = base.PresencePenalty
			p.FrequencyPenalty = base.FrequencyPenalty
			p.Seed = base.Seed
			p.MaxCompletionTokens = base.MaxCompletionTokens
		}
		return nil
	}
}

// withOpenAIMessages populates the chat params with system + user/assistant messages.
func withOpenAIMessages(
	messages []memory.Message,
	instructions string,
) func(*openai.ChatCompletionNewParams) error {
	return func(p *openai.ChatCompletionNewParams) error {
		// start with system instruction
		p.Messages = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(instructions),
		}
		// append each message
		for _, msg := range messages {
			text := msg.Text.String()
			if msg.Role == memory.ModelRole {
				p.Messages = append(p.Messages, openai.AssistantMessage(text))
			} else {
				p.Messages = append(p.Messages, openai.UserMessage(text))
			}
		}

		return nil
	}
}

type openAIChatService[T openai.ChatCompletionNewParams] struct {
	defaultRequestConfig *RequestConfig
	*llmRequestService[openai.ChatCompletionNewParams]
	openaiClient *openai.Client
}

func (o *openAIChatService[T]) buildRequestParams(opts ...func(*openai.ChatCompletionNewParams) error) error {
	return o.buildParams(opts...)
}

// request sends a chat completion request to OpenAI and returns the generated text.
func (o *openAIChatService[T]) request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	// rate limiting / preparation
	t, err := o.initRequest(ctx, messages)
	if err != nil {
		return "", err
	}
	defer o.logTime(t())

	// execute non-streaming request
	resp, err := o.openaiClient.Chat.Completions.New(ctx, *o.requestTypedConfig)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.Errorf("openai returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

func (c openAIClient) request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	t, err := c.llm.request(ctx, messages)
	if err != nil {
		return "", err
	}

	c.requestConfig.Messages = c.prepare(messages)

	var (
		done   bool
		tries  int
		apiErr openai.ErrorObject
		res    *openai.ChatCompletion
		result string
		retry  time.Duration = 0
	)

	for !done {
		// Generate a retry time in case of a request failure.

		sleep := getWaitTime(defaultRetryInterval)

		// Make a new request context for every retry.

		rCtx, rCancel := context.WithCancelCause(ctx)

		defer rCancel(ErrDispatchContextCancelled)

		if tries >= maxRequestRetries {
			done = true
			continue
		}

		select {
		case <-rCtx.Done():
			return "", rCtx.Err()
		default:
			res, err = c.client.Chat.Completions.New(ctx, *c.requestConfig)
			if err != nil {
				ok := errors.As(err, &apiErr)
				if ok {
					if apiErr.Code == "500" || apiErr.Code == "503" {
						c.logger.Warnf("API error: %s %s", apiErr.Code, apiErr.Message)
						c.logger.Debugf("Retrying in %s", sleep)
						retry = sleep
					}
				}

				if retry == 0 {
					done = true
				}

				select {
				case <-rCtx.Done():
					return "", rCtx.Err()
				case <-time.After(retry):
					rCancel(ErrDispatchContextCancelled)
				}

				continue
			}
		}

		result = res.Choices[0].Message.Content

		done = true

		tries++
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "", ErrDispatchContextCancelled
	case err != nil:
		return "", err
	}

	c.logTime(t())

	return result, nil
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

func (c openAIClient) setTemperature(t float64) float64 {
	return t
}
