package memory

import "codeberg.org/n30w/jasima/chat"

type LogographyGeneration map[string]string

type SpecificationGeneration map[chat.Layer]chat.Content

type DictionaryGeneration map[string]string

// Generation contains all generational information related to a single
// iteration of a conlang's development.
type Generation struct {
	Transcript     []Message
	Logography     LogographyGeneration
	Specifications SpecificationGeneration
	Dictionary     DictionaryGeneration
}
