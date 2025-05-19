package llms

import (
	"context"
	"fmt"
	"net/http"
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

const (
	defaultOllamaUrl           = "http://localhost:11434"
	defaultOllamaSleepDuration = time.Second * 2
)

type ollamaRequestClientType int

func newOllamaRequestClientType(i int) (ollamaRequestClientType, error) {
	o := ollamaRequestClientType(i)
	err := o.validate()
	return invalidRequestClientType, err
}

func (oc ollamaRequestClientType) validate() error {
	if oc > 1 || oc < 0 {
		return errors.Errorf("invalid Ollama client type %d", oc)
	}

	return nil
}

const (
	invalidRequestClientType                         = -1
	useHttpClientRequest     ollamaRequestClientType = iota
	useOllamaClientRequest
)

type ollamaUseStreaming bool

type OllamaModelConfig struct {
	OllamaClientMode   int
	OllamaUseStreaming bool
}

type Ollama struct {
	*llm[ol.ChatRequest]
	logger       *log.Logger
	hc           *network.HttpRequestClient[ol.ChatResponse]
	olClient     *ol.Client
	clientMode   ollamaRequestClientType
	useStreaming ollamaUseStreaming
}

// NewOllama creates a new Ollama LLM service. `url` is the URL of the server
// hosting the Ollama instance. If URL is nil, the default instance URL is used.
func NewOllama(_ string, mc ModelConfig, l *log.Logger) (
	*Ollama,
	error,
) {
	// TODO make this look better and more readable.

	var err error
	var u *url.URL

	if mc.ApiUrl == "" {
		u, err = url.Parse(defaultOllamaUrl)
		if err != nil {
			return nil, err
		}
	} else {
		u, err = url.Parse(mc.ApiUrl)
		if err != nil {
			return nil, err
		}
	}

	u2 := *u
	olc := ol.NewClient(&u2, &http.Client{})

	u.Path = "/api/chat"

	hc, err := network.NewHttpRequestClient[ol.ChatResponse](u, l)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create http request client")
	}

	newConf := mc
	g := defaultOllamaRequestConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	nl, err := newLLM[ol.ChatRequest](newConf, l)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ollama client")
	}

	nl.sleepDuration = defaultOllamaSleepDuration
	nl.apiUrl = u

	cm, err := newOllamaRequestClientType(mc.Configs.OllamaClientMode)
	if err != nil {
		return nil, err
	}

	us := ollamaUseStreaming(mc.Configs.OllamaUseStreaming)

	return &Ollama{
		llm:          nl,
		logger:       l,
		hc:           hc,
		olClient:     olc,
		clientMode:   cm,
		useStreaming: us,
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

	b := bool(c.useStreaming)

	return &ol.ChatRequest{
		Model:     c.model.String(),
		Stream:    &b,
		Options:   m,
		KeepAlive: &ol.Duration{Duration: 1 * time.Minute},
	}, nil
}

func (c Ollama) Request(
	ctx context.Context,
	messages []memory.Message,
	rc *RequestConfig,
) (
	string,
	error,
) {
	var err error

	c.config, err = c.buildRequestParams(rc)
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
	t, err := c.llm.request(ctx, messages)
	if err != nil {
		return "", err
	}

	defer c.logTime(t())

	c.config.Messages = c.prepare(messages)

	if c.clientMode == useOllamaClientRequest || c.useStreaming {
		return c.olClientRequest(ctx)
	}

	return c.httpRequest(ctx)
}

func (c Ollama) olClientRequest(ctx context.Context) (string, error) {
	var result strings.Builder

	respFunc := func(resp ol.ChatResponse) error {
		select {
		case <-ctx.Done():
			return ErrDispatchContextCancelled
		default:
			result.WriteString(resp.Message.Content)
		}

		return nil
	}

	select {
	case <-ctx.Done():
		return "", ErrDispatchContextCancelled
	default:
		select {
		case <-ctx.Done():
			return "", ErrDispatchContextCancelled
		default:
			err := c.olClient.Chat(ctx, c.config, respFunc)
			if err != nil {
				return "", err
			}

			return result.String(), nil
		}
	}
}

func (c Ollama) httpRequest(ctx context.Context) (string, error) {
	request, err := c.hc.PreparePost(c.config)
	if err != nil {
		return "", err
	}

	select {
	case <-ctx.Done():
		return "", ErrDispatchContextCancelled
	default:
		res, err := request(ctx)
		if err != nil {
			return "", err
		}

		return res.Message.Content, nil
	}
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

func RequestTypedOllama[T any](
	ctx context.Context,
	messages []memory.Message,
	llm *Ollama,
	rc *RequestConfig,
) (string, error) {
	var (
		err    error
		result string
	)

	_, err = lookupType[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to lookup type")
	}

	llm.config, err = llm.buildRequestParams(rc)
	if err != nil {
		return "", errors.Wrap(err, "failed to build request params")
	}

	s, err := utils.GenerateJsonSchema[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate json schema")
	}

	llm.config.Format = s

	result, err = llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to make typed ollama request")
	}

	result = removeThinkingTags(result)

	return strings.TrimSpace(result), nil
}
