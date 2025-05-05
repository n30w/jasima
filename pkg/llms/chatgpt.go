package llms

import (
	"context"
	"fmt"
	"reflect"

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
	var (
		v      T
		err    error
		result string
	)

	t := reflect.TypeOf(v)
	s, err := schemas.lookup(t)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve schema for ChatGPT")
	}

	llm.cfg = llm.buildRequestParams(nil)
	llm.cfg.ResponseFormat = openai.
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
