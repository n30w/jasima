package llms

import (
	"reflect"
	"sync"

	"github.com/openai/openai-go"
	"github.com/pkg/errors"
	"google.golang.org/genai"
)

type schemaRegistry struct {
	mu       sync.RWMutex
	registry map[reflect.Type]*schema
}

type schema struct {
	gemini *genai.Schema
	openai *openai.ResponseFormatJSONSchemaJSONSchemaParam
}

func newSchemaRegistry() *schemaRegistry {
	return &schemaRegistry{
		registry: make(map[reflect.Type]*schema),
	}
}

func (g *schemaRegistry) register(v reflect.Type, schema *schema) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.registry[v] = schema
}

func (g *schemaRegistry) lookup(v reflect.Type) (*schema, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s, ok := g.registry[v]
	if !ok {
		return nil, errors.New("lookup type not in gemini schema registry")
	}

	return s, nil
}

func lookupType[T any]() (*schema, error) {
	var v T
	t := reflect.TypeOf(v)
	s, err := schemas.lookup(t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve schema")
	}
	return s, nil
}
