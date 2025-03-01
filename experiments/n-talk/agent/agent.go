package main

// Memory is a memory storage. It supports saving and retrieving messages
// from a memory storage.
type Memory interface {
	Save(s string) error
	Retrieve(e int) error
	All() []*Message
	IsEmpty() bool
}
