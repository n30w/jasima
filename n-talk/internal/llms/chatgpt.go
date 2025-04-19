package llms

import (
	"context"
	"fmt"

	"codeberg.org/n30w/jasima/n-talk/internal/memory"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAIChatGPT struct {
	*llm
	chatGptClient                      *openai.Client
	chatGptCompletionParams            *openai.ChatCompletionNewParams
	chatGptCompletionMessageParamUnion []openai.ChatCompletionMessageParamUnion
}

func NewOpenAIChatGPT(
	apiKey string,
	mc ModelConfig,
) (*OpenAIChatGPT, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0)
	messages = append(messages, openai.SystemMessage(""))

	gpt := &OpenAIChatGPT{
		llm: &llm{
			model: ProviderChatGPT,
		},
		chatGptClient: openai.NewClient(option.WithAPIKey(apiKey)),
		chatGptCompletionParams: &openai.ChatCompletionNewParams{
			Seed:                openai.Int(1),
			Model:               openai.F(openai.ChatModelGPT4o),
			MaxCompletionTokens: openai.Int(2000),
			Temperature:         openai.Float(mc.Temperature),
			Messages:            openai.F(messages),
		},
	}

	return gpt, nil
}

func (c OpenAIChatGPT) Request(
	ctx context.Context,
	messages []memory.Message,
	prompt string,
) (string, error) {
	contents := c.prepare(messages)

	c.chatGptCompletionParams.Messages = openai.F(contents)

	result, err := c.chatGptClient.Chat.Completions.New(
		ctx,
		*c.chatGptCompletionParams,
	)
	if err != nil {
		return "", err
	}

	return result.Choices[0].Message.Content, nil
}

func (c OpenAIChatGPT) prepare(
	messages []memory.Message,
) []openai.ChatCompletionMessageParamUnion {
	contents := make([]openai.ChatCompletionMessageParamUnion, 0)

	// Append the current System Message to contents.

	contents = append(contents, c.chatGptCompletionMessageParamUnion[0])

	l := len(messages)

	if l != 0 {
		for i, v := range messages {

			var content openai.ChatCompletionMessageParamUnion

			content = openai.UserMessage(v.Text)

			if v.Role.String() == "model" {
				content = openai.AssistantMessage(v.Text)
			}

			contents[i] = content
		}
	}

	return contents
}

func (c OpenAIChatGPT) String() string {
	return fmt.Sprintf("OpenAI ChatGPT %s", c.model)
}
