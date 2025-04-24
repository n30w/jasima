package agent

// Command defines integer values that represent server commands.
// Server commands are sent to clients via messages. Clients must
// adhere to server commands.
type Command int32

func (c Command) Int32() int32 {
	return int32(c)
}

const (
	NoCommand Command = 0

	// AppendInstructions appends additional initial instructions
	// for an LLM model.
	AppendInstructions Command = 2

	// SetInstructions changes the client's instructions to new
	// ones from the message body.
	SetInstructions Command = 3

	// ResetInstructions resets a client's instructions to its original
	// state.
	ResetInstructions Command = 5

	// SendInitialMessage makes a client send an initial message to its
	// peers.
	SendInitialMessage Command = 4

	// Latch requires a client go into `latch` mode.
	Latch Command = 10

	// Unlatch requires a client go into `unlatch` mode.
	Unlatch Command = -10

	// ClearMemory requires a client to clear its entire memory.
	ClearMemory Command = -20
)

func (c Command) String() string {
	switch c {
	case NoCommand:
		return "NO_COMMAND"
	case AppendInstructions:
		return "APPEND_INSTRUCTIONS"
	case SetInstructions:
		return "SET_INSTRUCTIONS"
	case ResetInstructions:
		return "RESET_INSTRUCTIONS"
	case SendInitialMessage:
		return "SEND_INITIAL_MESSAGE"
	case Latch:
		return "LATCH"
	case Unlatch:
		return "UNLATCH"
	case ClearMemory:
		return "CLEAR_MEMORY"
	default:
		return "UNKNOWN COMMAND"
	}
}
