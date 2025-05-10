package llms

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

type llm struct {
	// model is the name of the model.
	model LLMProvider

	// instructions is a system instruction.
	instructions string

	defaultConfig *RequestConfig

	logger *log.Logger
}

func newLLM(mc ModelConfig, l *log.Logger) (*llm, error) {
	err := mc.validate()
	if err != nil {
		return nil, errors.Wrap(err, "invalid model config")
	}

	l.Debugf("Creating new LLM instance with these options: %+v", mc)

	return &llm{
		model:         mc.Provider,
		instructions:  mc.Instructions,
		defaultConfig: &mc.RequestConfig,
		logger:        l,
	}, nil
}

func (l *llm) SetInstructions(s string) {
	l.instructions = s
}

// setTemperature remaps a temperature value, such as 0.5, to a model specific
// value. Gemini and Deepseek use a scale from 0.0 to 2.0, rather than the
// typical 0.0 to 1.0. This function lets config parameters maintain a
// consistent input mapping of 0.0 to 1.0 rather than having two
// different mappings.
func (l *llm) setTemperature(t float64) float64 {
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
	s := "INVALID PROVIDER"

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
	}

	return s
}

type ModelConfig struct {
	Provider     LLMProvider
	Instructions string
	Initialize   string
	Temperature  float64
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

func buildString(strs ...string) string {
	var sb strings.Builder

	for _, str := range strs {
		sb.WriteString("\n")
		sb.WriteString(str)
	}

	return sb.String()
}

var thinkTagPattern = regexp.MustCompile(`(?s)<think>.*?</think>\n?`)

func removeThinkingTags(response string) string {
	return thinkTagPattern.ReplaceAllString(response, "")
}
