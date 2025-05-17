package llms

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"codeberg.org/n30w/jasima/pkg/memory"
)

type testResponse struct {
	Text string `json:"text"`
}

type args struct {
	ctx      context.Context
	messages []memory.Message
	llm      *Ollama
}

var testSchemas *schemaRegistry

func init() {
	testSchemas = newSchemaRegistry()
	schemas.register(reflect.TypeOf(testResponse{}), nil)
}

func buildTestOllama(t *testing.T) (*Ollama, error) {
	mc := ModelConfig{
		Provider:      ProviderOllama,
		Instructions:  "instructions are added later in test cases.",
		RequestConfig: *defaultOllamaConfig,
	}

	l := log.New(os.Stdout)

	llm, err := NewOllama(nil, mc, l)
	if err != nil {
		t.Fatal(err)
	}

	return llm, nil
}

func TestRequestTypedOllama(t *testing.T) {
	llm, err := buildTestOllama(t)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		args         args
		want         string
		wantErr      bool
		instructions string
	}{
		{
			name: "receive default response",
			args: args{
				ctx: context.Background(),
				messages: []memory.Message{
					{
						Role: 0,
						Text: "Send back the words 'Hello world' in an" +
							" unprettified JSON object format.",
						Timestamp: time.Time{},
					},
				},
				llm: llm,
			},
			want:         `{"text": "Hello world"}`,
			wantErr:      false,
			instructions: "You are a helpful assistant.",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				tt.args.llm.instructions = tt.instructions
				got, err := RequestTypedOllama[testResponse](
					tt.args.ctx,
					tt.args.messages,
					tt.args.llm,
				)
				if (err != nil) != tt.wantErr {
					t.Errorf(
						"RequestTypedOllama() error = %v, wantErr %v",
						err,
						tt.wantErr,
					)
					return
				}
				if got != tt.want {
					t.Errorf(
						"RequestTypedOllama() got = %v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestRequestOllama(t *testing.T) {
	llm, err := buildTestOllama(t)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		args         args
		want         string
		wantErr      bool
		instructions string
	}{
		{
			name: "receive hello response",
			args: args{
				ctx: context.Background(),
				messages: []memory.Message{
					{
						Role: 0,
						Text: "Please send back the words 'I am ollama' only",
					},
				},
				llm: llm,
			},
			want:         "I am ollama",
			wantErr:      false,
			instructions: "You are a helpful assistant.",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				tt.args.llm.instructions = tt.instructions
				got, err := llm.Request(
					tt.args.ctx,
					tt.args.messages,
				)
				if (err != nil) != tt.wantErr {
					t.Errorf(
						"Ollama Request() error = %v, wantErr %v",
						err,
						tt.wantErr,
					)
					return
				}
				if got != tt.want {
					t.Errorf(
						"Ollama Request() got = %v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}
