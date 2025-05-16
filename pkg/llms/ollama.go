package llms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/charmbracelet/log"
	ol "github.com/ollama/ollama/api"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/network"
	"codeberg.org/n30w/jasima/pkg/utils"
)

const defaultOllamaUrl = "http://localhost:11434"

type Ollama struct {
	*llm
	cfg    *ol.ChatRequest
	logger *log.Logger
	hc     *network.HttpRequestClient[ol.ChatResponse]
	u      *url.URL
}

// NewOllama creates a new Ollama LLM service. `url` is the URL of the server
// hosting the Ollama instance. If URL is nil, the default instance URL is used.
func NewOllama(u *url.URL, mc ModelConfig, l *log.Logger) (
	*Ollama,
	error,
) {
	var err error

	if u == nil {
		u, err = url.Parse(defaultOllamaUrl)
		if err != nil {
			return nil, err
		}
	}

	u.Path = "/api/chat"

	hc, err := network.NewHttpRequestClient[ol.ChatResponse](u, l)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request client")
	}

	newConf := mc
	g := defaultOllamaConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	nl, err := newLLM(newConf, l)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ollama client")
	}

	return &Ollama{
		llm:    nl,
		logger: l,
		hc:     hc,
		u:      u,
	}, nil
}

func (c Ollama) buildRequestParams(rc *RequestConfig) (*ol.ChatRequest, error) {
	p := &ol.Options{
		Seed: int(c.defaultConfig.Seed),
		TopK: int(c.defaultConfig.TopK),
		TopP: float32(c.defaultConfig.TopP),
		Temperature: float32(
			c.setTemperature(
				c.defaultConfig.
					Temperature,
			),
		),
		PresencePenalty:  float32(c.defaultConfig.PresencePenalty),
		FrequencyPenalty: float32(c.defaultConfig.FrequencyPenalty),
	}

	if rc != nil {
		p = &ol.Options{
			Seed:             int(rc.Seed),
			TopK:             int(rc.TopK),
			TopP:             float32(rc.TopP),
			Temperature:      float32(c.setTemperature(rc.Temperature)),
			PresencePenalty:  float32(rc.PresencePenalty),
			FrequencyPenalty: float32(rc.FrequencyPenalty),
		}
	}

	m, err := utils.StructToMap(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert struct to map")
	}

	return &ol.ChatRequest{
		Model:     c.model.String(),
		Stream:    new(bool),
		Options:   m,
		KeepAlive: &ol.Duration{Duration: 1 * time.Minute},
	}, nil
}

func (c Ollama) Request(ctx context.Context, messages []memory.Message) (
	string,
	error,
) {
	var err error

	c.cfg, err = c.buildRequestParams(nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to build request params")
	}

	v, err := c.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to make ollama request")
	}

	s := removeThinkingTags(v)

	return strings.TrimSpace(s), nil
}

func (c Ollama) request(ctx context.Context, messages []memory.Message) (
	string,
	error,
) {
	if c.cfg == nil {
		return "", errNoConfigurationProvided
	}

	if len(messages) == 0 {
		return "", errNoContentsInRequest
	}

	c.cfg.Messages = c.prepare(messages)

	request, err := c.hc.PreparePost(c.cfg)
	if err != nil {
		return "", err
	}

	result, err := request(ctx)
	if err != nil {
		return "", err
	}

	return result.Message.Content, nil
}

func (c Ollama) prepare(messages []memory.Message) []ol.Message {
	// Add 1 for system instructions.
	l := len(messages) + 1

	contents := make([]ol.Message, l)

	contents[0] = ol.Message{
		Role:    "system",
		Content: c.instructions,
	}

	for _, v := range messages {
		r := "user"
		if v.Role == memory.ModelRole {
			r = "assistant"
		}
		content := ol.Message{
			Role:    r,
			Content: v.Text.String(),
		}

		contents = append(contents, content)
	}

	return contents
}

func (c Ollama) String() string {
	return fmt.Sprintf("Ollama %s", c.model)
}

func (c Ollama) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

func RequestTypedOllama[T any](
	ctx context.Context,
	messages []memory.Message,
	llm *Ollama,
) (string, error) {
	var (
		err    error
		result string
	)

	_, err = lookupType[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to lookup type")
	}

	llm.cfg, err = llm.buildRequestParams(nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to build request params")
	}

	s, err := utils.GenerateJsonSchema[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate json schema")
	}

	llm.cfg.Format = s

	result, err = llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to make typed ollama request")
	}

	result = removeThinkingTags(result)

	return strings.TrimSpace(result), nil
}
