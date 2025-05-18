package utils

import (
	"context"
	"fmt"
)

// SendWithContext wraps a select and panic recover function that sends a value
// to a channel while also respecting context.
func SendWithContext[T any](ctx context.Context, ch chan<- T, val T) (err error) {
	// Ignores error not checked.
	//nolint:errcheck
	defer func() error {
		if r := recover(); r != nil {
			return fmt.Errorf("channel send panic: %v", r)
		}
		return nil
	}()

	if ch == nil {
		return fmt.Errorf("channel is nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- val:
		return nil
	}
}
