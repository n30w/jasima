package llms

var defaultOpenAIConfig = &RequestConfig{
	Temperature:      0.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        4096,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultDeepseekConfig = &RequestConfig{
	Temperature:      1.72,
	Seed:             1,
	TopP:             1,
	MaxTokens:        4096,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultClaudeConfig = &RequestConfig{
	Temperature:      0.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        3000,
	FrequencyPenalty: 1.1,
	PresencePenalty:  1.3,
}

var defaultChatGPTConfig = &RequestConfig{
	Temperature:      0.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        6144,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultGeminiConfig = &RequestConfig{
	Temperature:      1.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        8192,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}
