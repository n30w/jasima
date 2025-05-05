package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"

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

	return &OpenAIChatGPT{o}, nil
}

func (c OpenAIChatGPT) Request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	c.cfg = c.buildRequestParams(nil)

	v, err := c.request(ctx, messages)
	if err != nil {
		return "", err
	}

	return v, nil
}

func (c OpenAIChatGPT) String() string {
	return fmt.Sprintf("Open AI %s", c.model)
}

func RequestTypedChatGPT[T any](
	ctx context.Context,
	messages []memory.Message,
	llm *OpenAIChatGPT,
) (string, error) {
	s := newOpenAIResponseSchema(utils.GenerateSchema[T]())

	llm.cfg = llm.buildRequestParams(nil)
	llm.cfg.ResponseFormat = openai.
		ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.
			ResponseFormatJSONSchemaParam{
			JSONSchema: s,
		},
	}

	result, err := llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to request typed ChatGPT")
	}

	return result, nil
}
