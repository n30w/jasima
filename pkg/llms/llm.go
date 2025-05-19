package llms

import (
	"context"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"
)

const (
	maxRequestRetries    = 3
	retryInterval        = 180 * time.Second
	defaultSleepDuration = 10 * time.Second
)

// llm is a base type for a Large Language Model. The generic `T` is the type
// for the request configuration, passed to the model specific library
// request method.
type llm[T any] struct {
	// model is the llm service provider.
	model LLMProvider

	// instructions are the system instructions for the model.
	instructions string

	// defaultConfig is the default configuration for a given model.
	defaultConfig *RequestConfig

	// config is the configuration used for each request of the LLM.
	config *T

	// apiUrl is the URL to use for API requests. If nil, a default
	// is used.
	apiUrl *url.URL

	// sleepDuration defines an upper bound for the total time
	// the LLM will wait before making a request to the service.
	// This duration differs based on model, but for these
	// purposes use the fastest time possible.
	sleepDuration time.Duration

	// logger is for logging data to the console.
	logger *log.Logger
}

// newLLM creates a new llm base.
func newLLM[T any](mc ModelConfig, l *log.Logger) (*llm[T], error) {
	err := mc.validate()
	if err != nil {
		return nil, errors.Wrap(err, "invalid model config")
	}

	u, err := url.Parse(mc.Url)
	if err != nil {
		return nil, err
	}

	return &llm[T]{
		model:         mc.Provider,
		instructions:  mc.Instructions,
		defaultConfig: &mc.RequestConfig,
		apiUrl:        u,
		logger:        l,
		sleepDuration: defaultSleepDuration,
	}, nil
}

func (l *llm[T]) SetInstructions(s string) {
	l.instructions = s
}

func (l *llm[T]) AppendInstructions(s string) {
	l.instructions = buildString(l.instructions, s)
}

func (l *llm[T]) String() string {
	return l.model.String()
}

// request checks that a request to an LLM service is ready to be made
// and also applies any rate limiting logic before returning. The return
// values include a timer which can be used to measure the total time made
// for a request and an error, which may be an ErrDispatchContextCancelled
// error the caller may choose to ignore.
func (l *llm[T]) request(ctx context.Context, messages []memory.Message) (
	func() time.Duration,
	error,
) {
	if l.config == nil {
		return nil, errNoConfigurationProvided
	}

	if len(messages) == 0 {
		return nil, errNoContentsInRequest
	}

	// Sleep for the prescribed time.

	l.logger.Debugf("Rate limiting for %s...", l.sleepDuration)

	select {
	case <-ctx.Done():
		l.logger.Warn("Dispatch context canceled")
		return nil, ErrDispatchContextCancelled
	case <-time.After(l.sleepDuration):
		l.logger.Debug("Dispatching message to LLM")
	}

	t := utils.Timer(time.Now())

	return t, nil
}

// setTemperature remaps a temperature value, such as 0.5, to a model specific
// value. GoogleGemini and Deepseek use a scale from 0.0 to 2.0, rather than the
// typical 0.0 to 1.0. This function lets config parameters maintain a
// consistent input mapping of 0.0 to 1.0 rather than having two
// different mappings.
func (l *llm[T]) setTemperature(t float64) float64 {
	switch l.model {
	case ProviderGoogleGemini_2_0_Flash:
		fallthrough
	case ProviderGoogleGemini_2_5_Flash:
		fallthrough
	case ProviderDeepseek:
		return t * 2
	default:
		return t
	}
}

func (l *llm[T]) logTime(t time.Duration) {
	l.logger.Debugf("LLM request roundtrip took %s", t)
}

type LLMProvider int

const (
	ProviderGoogleGemini_2_0_Flash LLMProvider = iota
	ProviderChatGPT
	ProviderDeepseek
	ProviderOllama
	ProviderClaude
	ProviderGoogleGemini_2_5_Flash
	InvalidProvider
)

func (l LLMProvider) String() string {
	var s string

	switch l {
	case ProviderGoogleGemini_2_0_Flash:
		s = "gemini-2.0-flash"
	case ProviderGoogleGemini_2_5_Flash:
		s = "gemini-2.5-flash-preview-04-17"
	case ProviderChatGPT:
		s = "gpt-4.1-mini-2025-04-14"
	case ProviderDeepseek:
		s = "deepseek-chat"
	case ProviderOllama:
		s = "qwen3:30b-a3b"
	case ProviderClaude:
		s = "claude-3-5-haiku-20241022"
	default:
		s = "INVALID PROVIDER"
	}

	return s
}

type ModelConfig struct {
	Provider     LLMProvider
	Instructions string
	Url          string
	RequestConfig
}

func (cfg *ModelConfig) validate() error {
	if cfg.Provider >= InvalidProvider {
		return errors.New("invalid LLM provider")
	}

	if cfg.Instructions == "" {
		return errors.New("missing instructions")
	}

	err := cfg.RequestConfig.validate()
	if err != nil {
		return err
	}

	return nil
}

// RequestConfig is a set of possible configurations for a request to an
// LLM service.
type RequestConfig struct {
	Temperature      float64
	Seed             int64
	TopP             float64
	TopK             float64
	MaxTokens        int64
	FrequencyPenalty float64
	PresencePenalty  float64
}

func (cfg RequestConfig) validate() error {
	if cfg.Temperature < 0.0 || cfg.Temperature > 1.0 {
		return errors.New("temperature must be between 0.0 and 1.0")
	}

	if cfg.TopP < 0.0 || cfg.TopP > 1.0 {
		return errors.New("top_p must be between 0.0 and 1.0")
	}

	if cfg.TopK < 0 {
		return errors.New("top_k must be non-negative")
	}

	if cfg.MaxTokens < 1 {
		return errors.New("max_tokens must be greater than 0")
	}

	if cfg.PresencePenalty < -2.0 || cfg.PresencePenalty > 2.0 {
		return errors.New("presence_penalty must be between -2.0 and 2.0")
	}

	if cfg.FrequencyPenalty < -2.0 || cfg.FrequencyPenalty > 2.0 {
		return errors.New("frequency_penalty must be between -2.0 and 2.0")
	}

	return nil
}

type llmError string

func (l llmError) Error() string {
	return string(l)
}

const (
	errNoContentsInRequest      llmError = "cannot send request with no content"
	errNoConfigurationProvided  llmError = "no configuration provided"
	ErrDispatchContextCancelled llmError = "dispatch context canceled"
)
