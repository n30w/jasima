package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/charmbracelet/log"
	ol "github.com/ollama/ollama/api"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"
)

const defaultOllamaUrl = "http://localhost:11434"

type Ollama struct {
	*llm
	cfg    *ol.ChatRequest
	client *ol.Client
	logger *log.Logger
	hc     *http.Client
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

	httpClient := &http.Client{Timeout: 0}

	// First check if Ollama is alive. Make a GET request. We don't care
	// about the value it returns. We only need to know if it errors.

	u.Path = "/api/version"

	_, err = httpClient.Get(u.String())
	if err != nil {
		return nil, errors.New("ollama is not running or invalid host URL")
	}

	l.Debug("Ollama is online.")

	// Then set up the chat API route.

	u.Path = "/api/chat"

	cfe := ol.NewClient(u, httpClient)

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
		client: cfe,
		logger: l,
		hc:     httpClient,
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
	var (
		err    error
		result ol.ChatResponse
		u      = defaultOllamaUrl + "/api/chat"
	)

	contents := c.prepare(messages)

	c.cfg.Messages = contents

	body, err := json.Marshal(c.cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal configuration")
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, u,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	res, err := c.hc.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to send request")
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	err = json.Unmarshal(resBody, &result)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response body")
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

	return strings.TrimSpace(result), nil
}
