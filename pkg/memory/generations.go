package memory

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"codeberg.org/n30w/jasima/pkg/chat"
)

type TranscriptGeneration map[chat.Layer][]Message

func (t TranscriptGeneration) Copy() TranscriptGeneration {
	newMap := make(TranscriptGeneration)

	for k, v := range t {
		m := make([]Message, len(v))
		copy(m, v)
		newMap[k] = m
	}

	return newMap
}

type LogographyGeneration map[string]string

func (l LogographyGeneration) Copy() LogographyGeneration {
	newMap := make(LogographyGeneration)
	maps.Copy(newMap, l)
	return newMap
}

type SpecificationGeneration map[chat.Layer]chat.Content

func (s SpecificationGeneration) Copy() SpecificationGeneration {
	newMap := make(SpecificationGeneration)
	maps.Copy(newMap, s)
	return newMap
}

type SpecificationUpdate struct {
	Specification string `json:"specification" jsonschema_description:"Update"`
	Explanation   string `json:"explanation" jsonschema_description:"Explanation of update"`
}

type DictionaryGeneration map[string]DictionaryEntry

func (d DictionaryGeneration) Copy() DictionaryGeneration {
	newMap := make(DictionaryGeneration)
	maps.Copy(newMap, d)
	return newMap
}

func (d DictionaryGeneration) String() string {
	var sb strings.Builder
	for _, entry := range d {
		w := fmt.Sprintf("%s:%s\n", entry.Word, entry.Definition)
		sb.WriteString(w)
	}

	return sb.String()
}

func (d DictionaryGeneration) MarshalJSON() ([]byte, error) {
	dictArr := make([]DictionaryEntry, 0)
	for _, entry := range d {
		if entry.Word != "" {
			dictArr = append(dictArr, entry)
		}
	}
	s, _ := json.Marshal(dictArr)
	return s, nil
}

type DictionaryEntry struct {
	chat.DictionaryEntryResponse

	// Logogram is the logogram of the word.
	Logogram string `json:"logogram,omitempty" jsonschema_description:"Dictionary entry logogram"`
}

// Generation contains all generational information related to a single
// iteration of a conlang's development.
type Generation struct {
	Transcript     TranscriptGeneration    `json:"transcript"`
	Logography     LogographyGeneration    `json:"logography"`
	Specifications SpecificationGeneration `json:"specifications"`
	Dictionary     DictionaryGeneration    `json:"dictionary"`
}
