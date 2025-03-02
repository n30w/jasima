package main

import (
	"codeberg.org/n30w/jasima/n-talk/memory"
)

// Memory is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type Memory interface {
	Save(s string) error
	Retrieve(e int) error
	All() []*memory.Message
}
