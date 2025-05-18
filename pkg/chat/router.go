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
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		mCtx, mCancel := context.WithCancel(ctx)
		defer mCancel()
		for msg := range ch {
			select {
			case <-mCtx.Done():
				return nil
			default:
				for _, f := range routes {
					err := f(mCtx, msg)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}
