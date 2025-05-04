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

type OpenAIChatGPT[T any] struct {
	*openAIClient
}

func NewOpenAIChatGPT(
	apiKey string,
	mc ModelConfig,
	logger *log.Logger,
) func() (*OpenAIChatGPT, error) {
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

	c := &OpenAIChatGPT[T]{o}

	c.llm.responseFormat = ResponseFormatJson

	return c, nil
}

func (c OpenAIChatGPT[T]) Request(
	ctx context.Context,
	messages []memory.Message,
	_ string,
) (string, error) {
	checkResponseFormat := func(cfg *openai.ChatCompletionNewParams) {
		if c.responseFormat == ResponseFormatJson {
			s := newOpenAIResponseSchema(utils.GenerateSchema[T]())
			cfg.ResponseFormat = openai.
				ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &openai.
					ResponseFormatJSONSchemaParam{
					JSONSchema: s,
				},
			}
		}
	}

	v, err := c.request(ctx, messages, checkResponseFormat)
	if err != nil {
		return "", err
	}

	var r T

	if c.responseFormat == ResponseFormatJson {
		r, err = unmarshal[T](v)
		if err != nil {
			return v, errors.Wrap(
				err,
				"openai client failed to unmarshal response",
			)
		}
	}

	return r, nil
}

func (c OpenAIChatGPT[T]) String() string {
	return fmt.Sprintf("Open AI %s", c.model)
}
