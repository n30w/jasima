package chat

import "codeberg.org/n30w/jasima/n-talk/internal/commands"

type Name string

func (n Name) String() string {
	return string(n)
}

const SystemName Name = "SYSTEM"

type Content string

type Layer int32

const (
	SystemLayer Layer = iota
	PhoneticsLayer
	GrammarLayer
	DictionaryLayer
	LogographyLayer
	UnknownLayer
)

func (l Layer) Int32() int32 {
	return int32(l)
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
	default:
		return UnknownLayer
	}
}

// NewPbMessage constructs a new protobuf Message.
func NewPbMessage(
	sender, receiver Name,
	content string,
	layer Layer,
	cmd ...commands.Command,
) *Message {
	m := &Message{
		Sender:   sender.String(),
		Receiver: receiver.String(),
		Content:  content,
		Command:  -1,
		Layer:    layer.Int32(),
	}

	if len(cmd) > 0 {
		m.Command = cmd[0].Int32()
	}

	return m
}
