package llms

import (
	"context"
	"fmt"

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
	_ string,
) (string, error) {
	contents := c.prepare(messages)

	params := c.buildRequestParams(nil)

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model.String(),
		contents,
		params,
	)
	if err != nil {
		return "", errors.Wrap(err, "gemini client failed to make request")
	}

	d, err := unmarshal[defaultAgentResponse](result.Text())
	if err != nil {
		return "", errors.Wrap(
			err,
			"failed to unmarshal agent response",
		)
	}

	return d.Response, nil
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
