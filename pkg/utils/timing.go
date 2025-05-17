package utils

import (
	"time"
)

// Timer makes a timer that starts at a specified time. To stop the timer,
// call the function that is returned. The duration is truncated to the
// thousandth's place of a second.
func Timer(start time.Time) func() time.Duration {
	return func() time.Duration {
		return time.Since(start).Truncate(time.Millisecond)
	}
}
