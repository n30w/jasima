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
	chatGptClient           *openai.Client
	chatGptCompletionParams *openai.ChatCompletionNewParams
}

func (c OpenAIChatGPT) SetInstructions(s string) {
	c.instructions = s
}

func (c OpenAIChatGPT) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

func NewOpenAIChatGPT(
	apiKey string,
	mc ModelConfig,
) (*OpenAIChatGPT, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0)
	messages = append(messages, openai.SystemMessage(mc.Instructions))

	c := openai.NewClient(option.WithAPIKey(apiKey))

	gpt := &OpenAIChatGPT{
		llm: &llm{
			model: ProviderChatGPT,
		},
		chatGptClient: &c,
		chatGptCompletionParams: &openai.ChatCompletionNewParams{
			Seed:                openai.Int(1),
			Model:               openai.ChatModelGPT4o,
			MaxCompletionTokens: openai.Int(10000),
			Temperature:         openai.Float(mc.Temperature),
			Messages:            messages,
		},
	}

	return gpt, nil
}

func (c OpenAIChatGPT) Request(
	ctx context.Context,
	messages []memory.Message,
	_ string,
) (string, error) {
	contents := c.prepare(messages)

	c.chatGptCompletionParams.Messages = contents

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

	instructions := openai.SystemMessage(c.instructions)

	contents = append(contents, instructions)

	l := len(messages)

	if l != 0 {
		for _, v := range messages {

			text := v.Text.String()

			var content openai.ChatCompletionMessageParamUnion

			content = openai.UserMessage(text)

			if v.Role.String() == "model" {
				content = openai.AssistantMessage(text)
			}

			contents = append(contents, content)
		}
	}

	return contents
}

func (c OpenAIChatGPT) String() string {
	return fmt.Sprintf("OpenAI ChatGPT %s", c.model)
}
