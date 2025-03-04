package main

import (
	"codeberg.org/n30w/jasima/n-talk/memory"
)

// Memory is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type Memory interface {

	// Save saves a message, using its role and text. A role of `0` saves as
	// "user". A role of `1` saves as "model".
	Save(role int, text string) error

	// Retrieve retrieves an `n` amount of messages from the storage. An `n`
	// less-than-or-equal-to zero returns all messages. Any `n` amount
	// less-than-or-equal-to the total number of memories returns `n` messages.
	Retrieve(n int) ([]memory.Message, error)
}
