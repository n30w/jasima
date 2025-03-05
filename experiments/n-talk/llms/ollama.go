package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"codeberg.org/n30w/jasima/n-talk/memory"
	ol "github.com/ollama/ollama/api"
)

type Ollama struct {
	*llm
	options *ol.Options
	// stream is whether to stream responses.
	stream *bool
	// url is the URL of the ollama server. By default, it should be
	// http://localhost:11434/api/generate.
	url        string
	httpClient http.Client
}

func NewOllama(model string, url string) *Ollama {
	s := false
	return &Ollama{
		llm: &llm{
			model: model,
		},
		options: &ol.Options{
			Temperature: 1.68,
		},
		stream:     &s,
		url:        url,
		httpClient: http.Client{Timeout: 10 * time.Second},
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

	l := len(messages)

	contents := make([]ol.Message, l)

	if l != 0 {
		for _, v := range messages {
			content := ol.Message{
				Role:    v.Role.String(),
				Content: v.Text,
			}

			contents = append(contents, content)
		}
	}

	return contents
}

func (c Ollama) String() string {
	return fmt.Sprintf("Ollama %s", c.model)
}
