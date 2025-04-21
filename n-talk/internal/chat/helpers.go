package chat

import "codeberg.org/n30w/jasima/n-talk/internal/commands"

type Name string

func (n Name) String() string {
	return string(n)
}

const SystemName Name = "SYSTEM"

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
	s := make([]Content, 0, len(l))
	s = append(s, l[PhoneticsLayer])
	s = append(s, l[GrammarLayer])
	s = append(s, l[DictionaryLayer])
	s = append(s, l[LogographyLayer])

	return s
}

// NewPbMessage constructs a new protobuf Message.
func NewPbMessage(
	sender, receiver Name,
	content Content,
	layer Layer,
	cmd ...commands.Command,
) *Message {
	m := &Message{
		Sender:   sender.String(),
		Receiver: receiver.String(),
		Content:  content.String(),
		Command:  -1,
		Layer:    layer.Int32(),
	}

	if len(cmd) > 0 {
		m.Command = cmd[0].Int32()
	}

	return m
}
