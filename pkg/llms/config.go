package llms

var defaultDeepseekRequestConfig = &RequestConfig{
	Temperature:      0.8,
	Seed:             1,
	TopP:             1,
	MaxTokens:        4096,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultClauseRequestConfig = &RequestConfig{
	Temperature:      0.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        3000,
	FrequencyPenalty: 1.1,
	PresencePenalty:  1.3,
}

var defaultChatGPTRequestConfig = &RequestConfig{
	Temperature:      0.75,
	Seed:             1,
	TopP:             1,
	MaxTokens:        6144,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultGeminiRequestConfig = &RequestConfig{
	Temperature:      0.8,
	Seed:             1,
	TopP:             1,
	MaxTokens:        8192,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}

var defaultOllamaRequestConfig = &RequestConfig{
	Temperature:      0.8,
	Seed:             1,
	TopP:             1,
	MaxTokens:        3000,
	FrequencyPenalty: 1.2,
	PresencePenalty:  1.2,
}
