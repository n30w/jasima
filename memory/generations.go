package memory

import (
	"encoding/json"
	"maps"

	"codeberg.org/n30w/jasima/chat"
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

type DictionaryGeneration map[string]DictionaryEntry

func (d DictionaryGeneration) Copy() DictionaryGeneration {
	newMap := make(DictionaryGeneration)
	maps.Copy(newMap, d)
	return newMap
}

func (d DictionaryGeneration) String() string {
	// There is probably a better way to do this.
	dictArr := make([]DictionaryEntry, len(d))

	for _, entry := range d {
		dictArr = append(dictArr, entry)
	}

	s, _ := json.Marshal(dictArr)

	return string(s)
}

type DictionaryEntry struct {
	Word       string `json:"word"`
	Definition string `json:"definition"`

	// Remove represents whether a word should be removed from the dictionary.
	// This is used when sending data to and from an agent. If an agent is
	// queried to remove an entry from the dictionary, this field would be
	// set to `true`.
	Remove bool `json:"remove"`
}

// Generation contains all generational information related to a single
// iteration of a conlang's development.
type Generation struct {
	Transcript     TranscriptGeneration    `json:"transcript"`
	Logography     LogographyGeneration    `json:"logography"`
	Specifications SpecificationGeneration `json:"specifications"`
	Dictionary     DictionaryGeneration    `json:"dictionary"`
}
