package main

import (
	"time"
)

func timer(start time.Time) func() time.Duration {
	return func() time.Duration {
		return time.Since(start)
	}
}
