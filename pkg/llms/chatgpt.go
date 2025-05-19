package llms

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/pkg/memory"
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
	g := defaultChatGPTRequestConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	withConfig := newOpenAIClient(
		apiKey,
		defaultChatGPTUrl,
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
	rc *RequestConfig,
) (string, error) {
	c.config = c.buildRequestParams(rc)

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
	rc *RequestConfig,
) (string, error) {
	var (
		err    error
		result string
	)

	s, err := lookupType[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve schema for ChatGPT")
	}

	llm.config = llm.buildRequestParams(rc)
	llm.config.ResponseFormat = openai.
		ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.
			ResponseFormatJSONSchemaParam{
			JSONSchema: *s.openai,
		},
	}

	result, err = llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to request typed ChatGPT")
	}

	return result, nil
}
