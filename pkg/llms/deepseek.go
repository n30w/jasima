package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/pkg/memory"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type Deepseek struct {
	*openAIClient
}

func NewDeepseek(apiKey string, mc ModelConfig, l *log.Logger) (
	*Deepseek,
	error,
) {
	newConf := mc
	g := defaultDeepseekConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	withConfig := newOpenAIClient(
		apiKey,
		"https://api.deepseek.com/v1",
		l,
	)

	nc, err := withConfig(newConf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Deepseek client")
	}

	return &Deepseek{nc}, nil
}

func (c Deepseek) Request(
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

func (c Deepseek) String() string {
	return fmt.Sprintf("Deepseek %s", c.model)
}
