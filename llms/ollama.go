package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"codeberg.org/n30w/jasima/memory"

	ol "github.com/ollama/ollama/api"
)

type Ollama struct {
	*llm
	options *ol.Options
	// stream is whether to stream responses.
	stream *bool
	// url is the URL of the ollama server. By default, it should be
	// http://localhost:11434/api/chat.
	url        string
	httpClient http.Client
}

func (c Ollama) SetInstructions(s string) {
	c.instructions = s
}

func (c Ollama) AppendInstructions(s string) {
	c.instructions = buildString(c.instructions, s)
}

// NewOllama creates a new Ollama LLM service. `url` is the URL of the server
// hosting the Ollama instance. If URL is nil, the default instance URL is used.
func NewOllama(url *url.URL, mc ModelConfig) (
	*Ollama,
	error,
) {
	var err error

	s := false

	ollamaUrl := url

	if url == nil {
		ollamaUrl, err = url.Parse("http://localhost:11434")
		if err != nil {
			return nil, err
		}
	}

	// First check if Ollama is alive. Make a GET request. We don't care
	// about the value it returns. We only need to know if it errors.

	ollamaUrl.Path = "/api/version"

	_, err = http.Get(ollamaUrl.String())
	if err != nil {
		return nil, errors.New("ollama is not running or invalid host URL")
	}

	// Then set up the chat API route.

	ollamaUrl.Path = "/api/chat"
	chatUrl := ollamaUrl.String()

	return &Ollama{
		llm: &llm{
			model:        ProviderOllama,
			instructions: mc.Instructions,
		},
		options: &ol.Options{
			Temperature: float32(mc.Temperature),
		},
		stream:     &s,
		url:        chatUrl,
		httpClient: http.Client{Timeout: 0},
	}, nil
}

func (c Ollama) Request(
	ctx context.Context,
	messages []memory.Message,
) (string, error) {
	contents := c.prepare(messages)

	options := make(map[string]any)

	options["temperature"] = c.options.Temperature

	ollamaRequest := ol.ChatRequest{
		Model:    c.model.String(),
		Stream:   c.stream,
		Messages: contents,
		Options:  options,
	}

	body, err := json.Marshal(ollamaRequest)
	if err != nil {
		return "", err
	}

	// fmt.Println(string(body))

	// Make a POST request to the server with parameters.

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.url,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var data ol.ChatResponse

	err = json.Unmarshal(resBody, &data)
	if err != nil {
		return "", err
	}

	return data.Message.Content, nil
}

func (c Ollama) prepare(messages []memory.Message) []ol.Message {
	// Add 1 for system instructions.
	l := len(messages) + 1

	contents := make([]ol.Message, l)

	contents[0] = ol.Message{
		Role:    "system",
		Content: c.instructions,
	}

	for i, v := range messages {
		content := ol.Message{
			Role:    v.Role.String(),
			Content: v.Text.String(),
		}

		// +1 because we added +1 to `l` to accommodate for system instructions.
		contents[i+1] = content
	}

	return contents
}

func (c Ollama) String() string {
	return fmt.Sprintf("Ollama %s", c.model)
}
