package domain

import "time"

// RealClock returns UTC wall clock time.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}
