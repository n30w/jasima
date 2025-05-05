package llms

import (
	"reflect"

	"github.com/openai/openai-go"
	"google.golang.org/genai"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"
)

var schemas *schemaRegistry

func init() {
	schemas = newSchemaRegistry()

	schemas.register(
		reflect.TypeOf(chat.AgentResponseText{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"response": {
						Type:        genai.TypeString,
						Description: "Your response",
					},
				},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "response",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[chat.AgentResponseText](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(memory.DictionaryEntries{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"entries": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type:        genai.TypeObject,
							Description: "",
							Properties: map[string]*genai.Schema{
								"word": {
									Type:        genai.TypeString,
									Description: "Dictionary entry word",
								},
								"definition": {
									Type:        genai.TypeString,
									Description: "Dictionary entry definition",
								},
								"remove": {
									Type:        genai.TypeBoolean,
									Description: "Whether to remove the word or not",
								},
							},
						},
					},
				},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "dictionary_entries",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[memory.DictionaryEntries](),
			},
		},
	)
}
