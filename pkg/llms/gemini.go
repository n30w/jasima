package llms

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/pkg/memory"

	"google.golang.org/genai"
)

type GoogleGemini struct {
	*llm[genai.GenerateContentConfig]
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
	g := defaultGeminiRequestConfig
	g.Temperature = mc.Temperature
	newConf.RequestConfig = *g

	l, err := newLLM[genai.GenerateContentConfig](newConf, logger)
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
		Temperature:     genai.Ptr(float32(c.setTemperature(c.defaultConfig.Temperature))),
		MaxOutputTokens: int32(c.defaultConfig.MaxTokens),
		Seed:            genai.Ptr(int32(c.defaultConfig.Seed)),
		// PresencePenalty:  genai.Ptr(float32(c.defaultConfig.PresencePenalty)),
		// FrequencyPenalty: genai.Ptr(float32(c.defaultConfig.FrequencyPenalty)),
	}

	if rc != nil {
		params = &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(c.setTemperature(rc.Temperature))),
			MaxOutputTokens: int32(rc.MaxTokens),
			Seed:            genai.Ptr(int32(rc.Seed)),
			// PresencePenalty:  genai.Ptr(float32(rc.PresencePenalty)),
			// FrequencyPenalty: genai.Ptr(float32(rc.FrequencyPenalty)),
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

	return params
}

func (c GoogleGemini) Request(
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

// request makes a request to the Gemini API. See Gemini API error codes here:
// https://ai.google.dev/gemini-api/docs/troubleshooting
func (c GoogleGemini) request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	t, err := c.llm.request(ctx, messages)
	if err != nil {
		return "", err
	}

	contents := c.prepare(messages)

	var (
		done   bool
		tries  int
		apiErr genai.APIError
		res    *genai.GenerateContentResponse
		result string
		retry  time.Duration = 0
	)

	for !done {
		// Generate a retry time in case of a request failure.
		sleep := getWaitTime(defaultRetryInterval)

		// Make a new request context for every retry.

		rCtx, rCancel := context.WithCancelCause(ctx)
		defer rCancel(ErrDispatchContextCancelled)

		if tries >= maxRequestRetries {
			done = true
			continue
		}

		select {
		case <-rCtx.Done():
			return "", rCtx.Err()
		default:
			res, err = c.client.Models.GenerateContent(
				rCtx,
				c.model.String(),
				contents,
				c.config,
			)
		}

		if err != nil {
			ok := errors.As(err, &apiErr)
			if ok {
				if apiErr.Code == 500 || apiErr.Code == 503 {
					c.logger.Warnf("API error: %d %s", apiErr.Code, apiErr.Message)
					c.logger.Debugf("Retrying in %s", sleep)
					retry = sleep
				}
			}

			if retry == 0 {
				done = true
			}

			select {
			case <-rCtx.Done():
				return "", rCtx.Err()
			case <-time.After(retry):
				rCancel(ErrDispatchContextCancelled)
			}

			continue
		}

		result = res.Text()
		done = true

		tries++
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "", ErrDispatchContextCancelled
	case err != nil:
		return "", err
	}

	c.logTime(t())

	return result, nil
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

func (c GoogleGemini) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

func RequestTypedGoogleGemini[T any](
	ctx context.Context,
	messages []memory.Message,
	llm *GoogleGemini,
	rc *RequestConfig,
) (string, error) {
	var (
		err    error
		result string
	)

	s, err := lookupType[T]()
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve schema for gemini")
	}

	llm.config = llm.buildRequestParams(rc)

	llm.config.ResponseMIMEType = "application/json"
	llm.config.ResponseSchema = s.gemini

	result, err = llm.request(ctx, messages)
	if err != nil {
		return "", errors.Wrap(err, "failed to make typed google gemini request")
	}

	return result, nil
}
