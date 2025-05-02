package llms

import (
	"strings"
)

type llm struct {
	// model is the name of the model.
	model LLMProvider

	// instructions is a system instruction.
	instructions string

	// responseFormat selects the type of response that is expected from the
	// LLM, like structured JSON output.
	responseFormat ResponseFormat
}

func (l *llm) SetInstructions(s string) {
	l.instructions = s
}

type ResponseFormat int

const (
	ResponseFormatText ResponseFormat = iota
	ResponseFormatJson
)

type LLMProvider int

const (
	ProviderGoogleGemini_2_0_Flash LLMProvider = iota
	ProviderChatGPT
	ProviderDeepseek
	ProviderOllama
	ProviderClaude
	ProviderGoogleGemini_2_5_Flash
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
		s = "qwen2.5:32b"
	case ProviderClaude:
		s = "claude-3-5-haiku-20241022"
	default:
		s = "unknown provider"
	}

	return s
}

type ModelConfig struct {
	Provider     LLMProvider
	Instructions string
	Temperature  float64
	Initialize   string
}

func buildString(strs ...string) string {
	var sb strings.Builder

	for _, str := range strs {
		sb.WriteString("\n")
		sb.WriteString(str)
	}

	return sb.String()
}
