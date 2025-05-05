package utils

import (
	"time"
)

func Timer(start time.Time) func() time.Duration {
	return func() time.Duration {
		return time.Since(start)
	}
}
