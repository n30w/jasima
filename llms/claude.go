package llms

import "fmt"

type Claude struct {
	*openAIClient
}

func NewClaude(
	apiKey string,
	mc ModelConfig,
) (*Claude, error) {
	withConfig := newOpenAIClient(apiKey, "https://api.anthropic.com/v1/")
	return &Claude{withConfig(mc)}, nil
}

func (c Claude) String() string {
	return fmt.Sprintf("Claude %s", c.model)
}
