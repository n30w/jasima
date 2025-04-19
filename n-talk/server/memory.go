package server

import (
	"bytes"
	"context"
	"strings"
	"text/template"
)

// LocalMemory wraps a MemoryService.
// It provides a working memory for the server.
type LocalMemory struct {
	MemoryService
}

// String serializes all memories into a string.
func (s LocalMemory) String() string {
	var builder strings.Builder

	t := template.New("t1")
	t, _ = t.Parse("{{.Sender}}: {{.Text}}\n")

	memories, _ := s.Retrieve(context.Background(), "", 0)

	for _, v := range memories {
		var buff bytes.Buffer
		_ = t.Execute(&buff, v)
		builder.Write(buff.Bytes())
	}

	return builder.String()
}
