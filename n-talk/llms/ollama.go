package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"codeberg.org/n30w/jasima/n-talk/memory"
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

func NewOllama(model string, url string, instructions string, temperature float64) *Ollama {

	s := false

	return &Ollama{
		llm: &llm{
			model:        model,
			instructions: instructions,
		},
		options: &ol.Options{
			Temperature: float32(temperature),
		},
		stream:     &s,
		url:        url,
		httpClient: http.Client{Timeout: 0},
	}
}

func (c *Ollama) Request(ctx context.Context, messages []memory.Message, prompt string) (string, error) {

	contents := c.prepare(messages)

	options := make(map[string]interface{})

	options["temperature"] = c.options.Temperature

	ollamaRequest := ol.ChatRequest{
		Model:    c.model,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
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

func (c *Ollama) prepare(messages []memory.Message) []ol.Message {

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
			Content: v.Text,
		}

		// +1 because we added +1 to `l` to accommodate for system instructions.
		contents[i+1] = content
	}

	return contents
}

func (c Ollama) String() string {
	return fmt.Sprintf("Ollama %s", c.model)
}
