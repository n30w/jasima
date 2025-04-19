package llms

// import "codeberg.org/n30w/jasima/n-talk/memory"

type llm struct {
	// model is the name of the model.
	model LLMProvider

	// instruction is a system instruction.
	instructions string
}

func (l *llm) SetInstructions(s string) {
	l.instructions = s
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
		s = "gemini-2.0-flash"
	case 1:
		s = "4o"
	case 2:
		s = "Deepseek"
	case 3:
		s = "qwen2.5:32b"
	}

	return s
}

type ModelConfig struct {
	Provider     int
	Instructions string
	Temperature  float64
	Initialize   string
}
