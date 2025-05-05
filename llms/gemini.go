package llms

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/memory"

	"google.golang.org/genai"
)

type GoogleGemini struct {
	*llm
	client *genai.Client
}

func NewGoogleGemini(
	apiKey string,
	mc ModelConfig,
	logger *log.Logger,
) (*GoogleGemini, error) {
	c, err := genai.NewClient(
		context.Background(),
		&genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		},
	)
	if err != nil {
		return nil, err
	}

	newConf := mc
	g := defaultGeminiConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	l, err := newLLM(newConf, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Gemini client")
	}

	return &GoogleGemini{
		llm:    l,
		client: c,
	}, nil
}

func (c GoogleGemini) buildRequestParams(rc *RequestConfig) *genai.GenerateContentConfig {
	params := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr(float32(c.defaultConfig.Temperature)),
		MaxOutputTokens: int32(c.defaultConfig.MaxTokens),
		Seed:            genai.Ptr(int32(c.defaultConfig.Seed)),
		// PresencePenalty:  genai.Ptr(float32(c.defaultConfig.PresencePenalty)),
		FrequencyPenalty: genai.Ptr(float32(c.defaultConfig.FrequencyPenalty)),
	}

	if rc != nil {
		params = &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(rc.Temperature)),
			MaxOutputTokens: int32(rc.MaxTokens),
			Seed:            genai.Ptr(int32(rc.Seed)),
			// PresencePenalty:  genai.Ptr(float32(rc.PresencePenalty)),
			FrequencyPenalty: genai.Ptr(float32(rc.FrequencyPenalty)),
		}
	}

	if c.model == ProviderGoogleGemini_2_5_Flash {
		// Gemini 2.5 lets you toggle whether thinking is on or off, via
		// the `ThinkingBudget` parameter. Setting it to 0 makes it not
		// think. Gemini 2.0 does not provide this capability.
		params.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr(int32(0)),
		}
		// Jack it up because we can.
		params.MaxOutputTokens = 32767
	}

	if c.responseFormat == ResponseFormatJson {
		params.ResponseMIMEType = "application/json"
		params.ResponseSchema = defaultGeminiResponse
	}

	return params
}

func (c GoogleGemini) Request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	v, err := c.request(ctx, messages, nil)
	if err != nil {
		return "", errors.Wrap(err, "LLM request failed")
	}

	return v, nil
}

type geminiConfigOpts = func(cfg *genai.GenerateContentConfig)

func (c GoogleGemini) request(
	ctx context.Context,
	messages []memory.Message,
	cfg *RequestConfig,
	opts ...geminiConfigOpts,
) (string, error) {
	p := c.buildRequestParams(cfg)

	contents := c.prepare(messages)

	for _, opt := range opts {
		opt(p)
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model.String(),
		contents,
		p,
	)
	if err != nil {
		return "", errors.Wrap(err, "gemini client failed to make request")
	}

	return result.Text(), nil
}

// prepare adheres memories to the `genai` library `content` type.
func (c GoogleGemini) prepare(messages []memory.Message) []*genai.Content {
	contents := make([]*genai.Content, 0)

	instructions := genai.NewContentFromText(c.instructions, genai.RoleModel)

	contents = append(contents, instructions)

	if len(messages) != 0 {
		for _, v := range messages {

			content := genai.NewContentFromText(
				v.Text.String(),
				genai.RoleUser,
			)

			if v.Role == memory.ModelRole {
				content = genai.NewContentFromText(
					v.Text.String(),
					genai.RoleModel,
				)
			}

			contents = append(contents, content)
		}
	}

	return contents
}

func (c GoogleGemini) String() string {
	return fmt.Sprintf("Google Gemini %s", c.model)
}

func (c GoogleGemini) SetInstructions(s string) {
	c.instructions = s
}

func (c GoogleGemini) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

var defaultGeminiResponse = &genai.Schema{
	Type:        genai.TypeObject,
	Description: agentResponseDescription,
	Properties: map[string]*genai.Schema{
		"response": {
			Type:        genai.TypeString,
			Description: "Your response",
		},
	},
}

type GoogleGeminiTyped[T any] struct {
	*GoogleGemini
	gsr *geminiSchemaRegistry
}

func NewGoogleGeminiTyped[T any](g *GoogleGemini) *GoogleGeminiTyped[T] {
	return &GoogleGeminiTyped[T]{
		GoogleGemini: g,
		gsr:          newGeminiSchemaRegistry(),
	}
}

func (gt GoogleGeminiTyped[T]) RequestTyped(
	ctx context.Context,
	messages []memory.Message,
	_ any,
) (T, error) {
	var (
		v      T
		err    error
		result string
	)

	t := reflect.TypeOf(v)
	s, err := gt.gsr.Lookup(t)
	if err != nil {
		return v, errors.Wrap(err, "failed to retrieve schema")
	}

	setMIMEType := func(cfg *genai.GenerateContentConfig) {
		cfg.ResponseMIMEType = "application/json"
	}

	setResponseSchema := func(cfg *genai.GenerateContentConfig) {
		cfg.ResponseSchema = s
	}

	result, err = gt.request(ctx, messages, nil, setMIMEType, setResponseSchema)
	if err != nil {
		return v, errors.Wrap(err, "GeminiTyped failed to make LLM request")
	}

	// Marshal JSON based on T.
	v, err = unmarshal[T](result)
	if err != nil {
		return v, errors.Wrap(err, "GeminiTyped failed to unmarshal JSON")
	}

	return v, nil
}

type geminiSchemaRegistry struct {
	mu       sync.RWMutex
	registry map[reflect.Type]*genai.Schema
}

func newGeminiSchemaRegistry() *geminiSchemaRegistry {
	g := &geminiSchemaRegistry{
		registry: make(map[reflect.Type]*genai.Schema),
	}

	// Register schema types.

	g.Register(reflect.TypeOf(defaultAgentResponse{}), &genai.Schema{
		Type:        genai.TypeObject,
		Description: agentResponseDescription,
		Properties: map[string]*genai.Schema{
			"response": {
				Type:        genai.TypeString,
				Description: "Your response",
			},
		},
	})

	return g
}

func (g *geminiSchemaRegistry) Register(v reflect.Type, schema *genai.Schema) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.registry[v] = schema
}

func (g *geminiSchemaRegistry) Lookup(v reflect.Type) (*genai.Schema, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s, ok := g.registry[v]
	if !ok {
		return nil, errors.New("schema not in gemini schema registry")
	}

	return s, nil
}
