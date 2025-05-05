package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/memory"
	"codeberg.org/n30w/jasima/utils"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/pkg/errors"
)

type OpenAIChatGPT struct {
	*openAIClient
}

func NewOpenAIChatGPT(
	apiKey string,
	mc ModelConfig,
	logger *log.Logger,
) (*OpenAIChatGPT, error) {
	newConf := mc
	g := defaultChatGPTConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	withConfig := newOpenAIClient(
		apiKey,
		ChatGPTBaseURL,
		logger,
	)

	o, err := withConfig(newConf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new ChatGPT client")
	}

	c := &OpenAIChatGPT{o}

	c.llm.responseFormat = ResponseFormatJson

	return c, nil
}

func (c OpenAIChatGPT) Request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	v, err := c.request(ctx, messages, nil)
	if err != nil {
		return "", err
	}

	return v, nil
}

func (c OpenAIChatGPT) String() string {
	return fmt.Sprintf("Open AI %s", c.model)
}

type ChatGPTTyped[T any] struct {
	*OpenAIChatGPT
}

func (t ChatGPTTyped[T]) RequestTyped(
	ctx context.Context,
	messages []memory.Message,
	_ any,
) (T, error) {
	var (
		v      T
		err    error
		result string
	)

	addSchema := func(cfg *openai.ChatCompletionNewParams) {
		s := newOpenAIResponseSchema(utils.GenerateSchema[T]())
		cfg.ResponseFormat = openai.
			ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.
				ResponseFormatJSONSchemaParam{
				JSONSchema: s,
			},
		}
	}

	result, err = t.request(ctx, messages, nil, addSchema)
	if err != nil {
		return v, errors.Wrap(err, "openai client failed to make typed LLM request")
	}

	v, err = unmarshal[T](result)
	if err != nil {
		return v, errors.Wrap(
			err,
			"openai client failed to unmarshal response",
		)
	}

	return v, nil
}
