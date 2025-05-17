package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/pkg/memory"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type Claude struct {
	*openAIClient
}

func NewClaude(
	apiKey string,
	mc ModelConfig,
	l *log.Logger,
) (*Claude, error) {
	if mc.Temperature > 1.0 {
		return nil, fmt.Errorf(
			"temperature %2f is not within range 0.0 to 1."+
				"0", mc.Temperature,
		)
	}

	newConf := mc
	g := defaultClaudeConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	withConfig := newOpenAIClient(
		apiKey,
		"https://api.anthropic.com/v1/",
		l,
	)

	o, err := withConfig(newConf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Claude client")
	}

	return &Claude{o}, nil
}

func (c Claude) Request(
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

func (c Claude) String() string {
	return fmt.Sprintf("Claude %s", c.model)
}
