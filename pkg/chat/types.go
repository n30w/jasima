package chat

import (
	"encoding/json"

	"codeberg.org/n30w/jasima/pkg/agent"
)

type Name string

func (n Name) String() string {
	return string(n)
}

type Content string

func (c Content) String() string {
	return string(c)
}

type Layer int32

const (
	SystemLayer Layer = iota
	PhoneticsLayer
	GrammarLayer
	DictionaryLayer
	LogographyLayer
	ChattingLayer
	UnknownLayer
)

func (l Layer) Int32() int32 {
	return int32(l)
}

func (l Layer) String() string {
	switch l {
	case SystemLayer:
		return "system"
	case PhoneticsLayer:
		return "phonetics"
	case GrammarLayer:
		return "grammar"
	case DictionaryLayer:
		return "dictionary"
	case LogographyLayer:
		return "logography"
	case ChattingLayer:
		return "chatting"
	default:
		return "Unknown Layer"
	}
}

func (l *Layer) UnmarshalJSON(b []byte) error {
	var s string

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	// switch s {
	// case 0:
	// 	*l = SystemLayer
	// case 1:
	// 	*l = PhoneticsLayer
	// case 2:
	// 	*l = GrammarLayer
	// case 3:
	// 	*l = DictionaryLayer
	// case 4:
	// 	*l = LogographyLayer
	// }

	switch s {
	default:
		*l = 100
	case "system":
		*l = SystemLayer
	case "phonetics":
		*l = PhoneticsLayer
	case "grammar":
		*l = GrammarLayer
	case "dictionary":
		*l = DictionaryLayer
	case "logography":
		*l = LogographyLayer
	case "chatting":
		*l = ChattingLayer
	}

	return nil
}

func (l Layer) MarshalJSON() ([]byte, error) {
	var s string

	switch l {
	case SystemLayer:
		s = "system"
	case PhoneticsLayer:
		s = "phonetics"
	case GrammarLayer:
		s = "grammar"
	case DictionaryLayer:
		s = "dictionary"
	case LogographyLayer:
		s = "logography"
	case ChattingLayer:
		s = "chatting"
	default:
		s = "unknown"
	}

	return json.Marshal(s)
}

func SetLayer(l int32) Layer {
	switch l {
	case 0:
		return SystemLayer
	case 1:
		return PhoneticsLayer
	case 2:
		return GrammarLayer
	case 3:
		return DictionaryLayer
	case 4:
		return LogographyLayer
	case 5:
		return ChattingLayer
	default:
		return UnknownLayer
	}
}

// LayerMessageSet defines content of a message for each layer.
type LayerMessageSet map[Layer]Content

func (l LayerMessageSet) ToSlice() []Content {
	s := make([]Content, 0)
	s = append(
		s,
		"", // Empty, since system layer is index 0
		l[PhoneticsLayer],
		l[GrammarLayer],
		l[DictionaryLayer],
		l[LogographyLayer],
	)

	return s
}

// NewPbMessage constructs a new protobuf Message.
func NewPbMessage(
	sender, receiver Name,
	content Content,
	layer Layer,
	cmd ...agent.Command,
) *Message {
	m := &Message{
		Sender:   sender.String(),
		Receiver: receiver.String(),
		Content:  content.String(),
		Command:  0,
		Layer:    layer.Int32(),
	}

	if len(cmd) > 0 {
		m.Command = cmd[0].Int32()
	}

	return m
}

type AgentResponseStop struct {
	Stop bool `json:"stop" jsonschema_description:"Indicates if you want to end the conversation"`
}

type AgentResponseText struct {
	Response string `json:"response" jsonschema_description:"Your response"`
}

type AgentLogogramIterationResponse struct {
	Name string `json:"name" jsonschema_description:"Logogram name"`
	Svg  string `json:"svg" jsonschema_description:"Logogram svg"`
	AgentResponseText
	AgentResponseStop
}

type AgentLogogramCritiqueResponse struct {
	Name string `json:"name" jsonschema_description:"Logogram name"`
	AgentResponseText
	AgentResponseStop
}

type AgentDictionaryWordsDetectionResponse struct {
	Words []string `json:"words" jsonschema_description:"Words in the dictionary from the text"`
}

type LogogramIteration struct {
	Generator AgentLogogramIterationResponse `json:"generator"`
	Adversary AgentLogogramCritiqueResponse  `json:"adversary"`
}

type DictionaryEntryResponse struct {
	Word       string `json:"word" jsonschema_description:"Dictionary entry word"`
	Definition string `json:"definition" jsonschema_description:"Dictionary entry definition"`

	// Remove represents whether a word should be removed from the dictionary.
	// This is used when sending data to and from an agent. If an agent is
	// queried to remove an entry from the dictionary, this field would be
	// set to `true`.
	Remove bool `json:"remove" jsonschema_description:"Remove word"`
}

type DictionaryEntriesResponse struct {
	Entries []DictionaryEntryResponse `json:"entries" jsonschema_description:"Dictionary entries"`
}
