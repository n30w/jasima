package llms

import (
	"fmt"
)

type OpenAIChatGPT struct {
	*openAIClient
}

func NewOpenAIChatGPT(
	apiKey string,
	mc ModelConfig,
) (*OpenAIChatGPT, error) {
	withConfig := newOpenAIClient(apiKey, ChatGPTBaseURL)
	return &OpenAIChatGPT{withConfig(mc)}, nil
}

func (c OpenAIChatGPT) String() string {
	return fmt.Sprintf("Open AI %s", c.model)
}
