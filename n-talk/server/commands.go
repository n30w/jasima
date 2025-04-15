package server

// Command defines integer values that represent server commands.
// Server commands are sent to clients via messages. Clients must
// adhere to server commands.
type Command int32

const (
	// AppendInstructions appends additional initial instructions
	// for an LLM model.
	AppendInstructions Command = 2

	// Latch requires a client go into `latch` mode.
	Latch Command = 10

	// Unlatch requires a client go into `unlatch` mode.
	Unlatch Command = -10

	// ClearMemory requires a client to clear its entire memory.
	ClearMemory Command = -20
)
