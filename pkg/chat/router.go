package chat

import (
	"context"
)

// BuildRouter is a builder function that creates a function that iterates
// over a channel of type `T`. The closure returned can be used as an
// asynchronous process to consume `T` from the channel and apply a list of
// functions, or “routes” to that specified type. The ORDER OF ROUTES MATTER!
// In other words, functions are executed in FIFO order.
func BuildRouter[T any](
	ch chan T,
	routes ...func(context.Context, T) error,
) func(errs chan<- error) {
	return func(errs chan<- error) {
		for msg := range ch {
			ctx := context.Background()
			for _, f := range routes {
				err := f(ctx, msg)
				if err != nil {
					errs <- err
				}
			}
		}
	}
}
