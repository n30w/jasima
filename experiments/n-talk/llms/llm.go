package llms

// import "codeberg.org/n30w/jasima/n-talk/memory"

type llm struct {
	model string
}

type LLMProvider int

const (
	ProviderGoogleGemini LLMProvider = iota
	ProviderChatGPT
	ProviderDeepseek
	ProviderOllama
)

func (l LLMProvider) String() string {

	s := "INVALID PROVIDER"

	switch l {
	case 0:
		s = "Google Gemini"
	case 1:
		s = "ChatGPT"
	case 2:
		s = "Deepseek"
	case 3:
		s = "Ollama"
	}

	return s
}
