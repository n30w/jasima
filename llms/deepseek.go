package llms

import "fmt"

type Deepseek struct {
	*openAIClient
}

func NewDeepseek(apiKey string, mc ModelConfig) (*Deepseek, error) {
	withConfig := newOpenAIClient(apiKey, "https://api.deepseek.com/v1")
	return &Deepseek{withConfig(mc)}, nil
}

func (c Deepseek) String() string {
	return fmt.Sprintf("Deepseek %s", c.model)
}
