package llms

import (
	"reflect"

	"github.com/openai/openai-go"
	"google.golang.org/genai"

	"codeberg.org/n30w/jasima/pkg/memory"
	"codeberg.org/n30w/jasima/pkg/utils"
)

var schemas *schemaRegistry

func init() {
	var (
		stopBool = &genai.Schema{
			Type:        genai.TypeBoolean,
			Description: "Indicates if you want to end the conversation",
		}
		responseTextString = &genai.Schema{
			Type:        genai.TypeString,
			Description: "Your response",
		}
	)

	schemas = newSchemaRegistry()

	schemas.register(
		reflect.TypeOf(memory.ResponseText{}), &schema{
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
				Schema: utils.GenerateSchema[memory.ResponseText](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(memory.ResponseDictionaryEntries{}), &schema{
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
				Schema: utils.GenerateSchema[memory.ResponseDictionaryEntries](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(memory.ResponseLogogramIteration{}), &schema{
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
					"response": responseTextString,
					"stop":     stopBool,
				},
				Required: []string{"name", "svg", "response", "stop"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "logogram_iteration",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[memory.ResponseLogogramIteration](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(memory.ResponseLogogramCritique{}), &schema{
			gemini: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"name": {
						Type:        genai.TypeString,
						Description: "The name of the logogram",
					},
					"response": responseTextString,
					"stop":     stopBool,
				},
				Required: []string{"name", "response", "stop"},
			},
			openai: &openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "logogram_critique",
				Strict: openai.Bool(true),
				Schema: utils.GenerateSchema[memory.ResponseLogogramCritique](),
			},
		},
	)

	schemas.register(
		reflect.TypeOf(memory.ResponseDictionaryWordsDetection{}), &schema{
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
				Schema: utils.GenerateSchema[memory.ResponseDictionaryWordsDetection](),
			},
		},
	)
}
