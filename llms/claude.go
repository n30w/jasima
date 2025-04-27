package llms

import "fmt"

type Claude struct {
	*openAIClient
}

func NewClaude(
	apiKey string,
	mc ModelConfig,
) (*Claude, error) {
	if mc.Temperature > 1.0 {
		return nil, fmt.Errorf(
			"temperature %2f is not within range 0.0 to 1."+
				"0", mc.Temperature,
		)
	}
	withConfig := newOpenAIClient(apiKey, "https://api.anthropic.com/v1/")
	return &Claude{withConfig(mc)}, nil
}

func (c Claude) String() string {
	return fmt.Sprintf("Claude %s", c.model)
}
