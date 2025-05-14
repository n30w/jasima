package llms

import (
	"reflect"

	"github.com/openai/openai-go"
	"google.golang.org/genai"

	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/utils"
)

var schemas *schemaRegistry

func init() {
	stopBool := &genai.Schema{
		Type:        genai.TypeBoolean,
		Description: "Indicates if the conversation is done",
	}

	schemas = newSchemaRegistry()

	schemas.register(
		reflect.TypeOf(chat.AgentResponseText{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"response": {
						Type:        genai.TypeString,
						Description: "Your response",
						Required:    []string{"response"},
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
		reflect.TypeOf(chat.DictionaryEntriesResponse{}), &schema{
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
							Required: []string{"word", "definition", "remove"},
						},
					},
				},
				Required: []string{"entries"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "dictionary_entries",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[chat.DictionaryEntriesResponse](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(chat.AgentLogogramIterationResponse{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"name": {
						Type:        genai.TypeString,
						Description: "The name of the logogram",
					},
					"svg": {
						Type:        genai.TypeString,
						Description: "The svg file for the logogram",
					},
					"reasoning": {
						Type:        genai.TypeString,
						Description: "The logogram change reasoning",
					},
					"stop": stopBool,
				},
				Required: []string{"name", "svg", "reasoning", "stop"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "logogram_iteration",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[chat.AgentLogogramIterationResponse](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(chat.AgentLogogramCritiqueResponse{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"name": {
						Type:        genai.TypeString,
						Description: "The name of the logogram",
					},
					"critique": {
						Type:        genai.TypeString,
						Description: "The logogram critique",
					},
					"stop": stopBool,
				},
				Required: []string{"name", "critique", "stop"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "logogram_critique",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[chat.AgentLogogramCritiqueResponse](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(chat.AgentDictionaryWordsDetectionResponse{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"words": {
						Type:        genai.TypeArray,
						Description: "Words in the dictionary from the text",
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
					},
				},
				Required: []string{"words"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "dictionary_words",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[chat.AgentDictionaryWordsDetectionResponse](),
			},
		},
	)
}
