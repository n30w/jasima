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

type ollamaUseGenerate bool

type OllamaModelConfig struct {
	OllamaClientMode   int
	OllamaUseStreaming bool
	OllamaUseGenerate  bool
}

type Ollama struct {
	*llmBase
	chatService     llmRequester[ol.ChatRequest]
	generateService llmRequester[ol.GenerateRequest]
	logger          *log.Logger
	clientMode      ollamaRequestClientType
	useStreaming    ollamaUseStreaming
	useGenerate     ollamaUseGenerate
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

	cm, err := newOllamaRequestClientType(mc.Configs.OllamaClientMode)
	if err != nil {
		return nil, err
	}

	us := ollamaUseStreaming(mc.Configs.OllamaUseStreaming)

	return &Ollama{
		chatService: &ollamaChatService[ol.ChatRequest]{
			olClient:             olc,
			httpClient:           hc,
			clientType:           cm,
			defaultRequestConfig: defaultOllamaRequestConfig,
			llmRequestService: &llmRequestService[ol.ChatRequest]{
				requestTypedConfig: &ol.ChatRequest{},
				sleepDuration:      defaultOllamaSleepDuration,
				apiUrl:             u,
			},
		},
		generateService: &ollamaGenerateService[ol.GenerateRequest]{
			defaultRequestConfig: defaultOllamaRequestConfig,
			llmRequestService: &llmRequestService[ol.GenerateRequest]{
				requestTypedConfig: &ol.GenerateRequest{},
				sleepDuration:      defaultOllamaSleepDuration,
				apiUrl:             u,
			},
		},
		logger:       l,
		clientMode:   cm,
		useStreaming: us,
	}, nil
}

func makeOllamaOptions(rc *RequestConfig, d *RequestConfig) (map[string]any, error) {
	p := &ol.Options{
		Seed:             int(d.Seed),
		TopK:             int(d.TopK),
		TopP:             float32(d.TopP),
		Temperature:      float32(d.Temperature),
		PresencePenalty:  float32(d.PresencePenalty),
		FrequencyPenalty: float32(d.FrequencyPenalty),
	}

	if rc != nil {
		p = &ol.Options{
			Seed:             int(rc.Seed),
			TopK:             int(rc.TopK),
			TopP:             float32(rc.TopP),
			Temperature:      float32(rc.Temperature),
			PresencePenalty:  float32(rc.PresencePenalty),
			FrequencyPenalty: float32(rc.FrequencyPenalty),
		}
	}

	m, err := utils.StructToMap(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert struct to map")
	}

	return m, nil
}

func withOllamaFormat[T any]() func(*ol.ChatRequest) error {
	return func(c *ol.ChatRequest) error {
		_, err := lookupType[T]()
		if err != nil {
			return errors.Wrap(err, "failed to lookup type")
		}

		b, err := utils.GenerateJsonSchema[T]()
		if err != nil {
			return errors.Wrap(err, "failed to generate json schema")
		}

		c.Format = b
		return nil
	}
}

func withOllamaMessages(
	messages []memory.Message,
	instructions string,
) func(*ol.ChatRequest) error {
	return func(c *ol.ChatRequest) error {
		// Add 1 for system instructions.
		l := len(messages) + 1

		contents := make([]ol.Message, l)

		contents[0] = ol.Message{
			Role:    "system",
			Content: instructions,
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

		c.Messages = contents

		return nil
	}
}

func withOllamaStreaming(s ollamaUseStreaming) func(*ol.ChatRequest) error {
	return func(c *ol.ChatRequest) error {
		b := bool(s)
		c.Stream = &b
		return nil
	}
}

func withOllamaRequestOptions(
	rc *RequestConfig,
	d *RequestConfig,
) func(*ol.ChatRequest) error {
	return func(req *ol.ChatRequest) error {
		m, err := makeOllamaOptions(rc, d)
		if err != nil {
			return err
		}

		req.Options = m

		return nil
	}
}

func withOllamaModel(m string) func(*ol.ChatRequest) error {
	return func(req *ol.ChatRequest) error {
		req.Model = m
		return nil
	}
}

type ollamaChatService[T ol.ChatRequest] struct {
	defaultRequestConfig *RequestConfig
	*llmRequestService[ol.ChatRequest]
	httpClient *network.HttpRequestClient[ol.ChatResponse]
	olClient   *ol.Client
	clientType ollamaRequestClientType
}

func (o *ollamaChatService[T]) buildRequestParams(opts ...func(*ol.ChatRequest) error) error {
	return o.buildParams(opts...)
}

func (o *ollamaChatService[T]) request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	t, err := o.initRequest(ctx, messages)
	if err != nil {
		return "", err
	}

	defer o.logTime(t())

	select {
	case <-ctx.Done():
		return "", ErrDispatchContextCancelled
	default:
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

		if *o.requestTypedConfig.Stream || o.clientType == useOllamaClientRequest {
			select {
			case <-ctx.Done():
				return "", ErrDispatchContextCancelled
			default:
				err = o.olClient.Chat(ctx, o.requestTypedConfig, respFunc)
				if err != nil {
					return "", err
				}
			}

			return result.String(), nil
		}

		request, err := o.httpClient.PreparePost(o.requestTypedConfig)
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
}

type ollamaGenerateService[T ol.GenerateRequest] struct {
	defaultRequestConfig *RequestConfig
	*llmRequestService[ol.GenerateRequest]
}

func (o *ollamaGenerateService[T]) buildRequestParams(opts ...func(*ol.GenerateRequest) error) error {
	return o.buildParams(opts...)
}

func (o *ollamaGenerateService[T]) request(ctx context.Context, messages []memory.Message) (
	string,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (c Ollama) Request(
	ctx context.Context,
	messages []memory.Message,
	rc *RequestConfig,
) (
	string,
	error,
) {
	err := c.chatService.buildRequestParams(
		withOllamaModel(c.model.String()),
		withOllamaStreaming(c.useStreaming),
		withOllamaRequestOptions(rc, defaultOllamaRequestConfig),
		withOllamaMessages(messages, c.instructions),
	)
	if err != nil {
		return "", err
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
	var s string
	var err error

	if c.useGenerate {
		s, err = c.generateService.request(ctx, messages)
		if err != nil {
			return "", err
		}
	} else {
		s, err = c.chatService.request(ctx, messages)
	}

	return s, err
}

func (c Ollama) String() string {
	return fmt.Sprintf("Ollama %s", c.model)
}

func (c Ollama) setTemperature(t float64) float64 {
	return t
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

	err = llm.chatService.buildRequestParams(
		withOllamaModel(llm.model.String()),
		withOllamaStreaming(llm.useStreaming),
		withOllamaRequestOptions(rc, defaultOllamaRequestConfig),
		withOllamaFormat[T](),
		withOllamaMessages(messages, llm.instructions),
	)
	if err != nil {
		return "", err
	}

	result, err = llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to make typed ollama request")
	}

	result = removeThinkingTags(result)

	return strings.TrimSpace(result), nil
}
