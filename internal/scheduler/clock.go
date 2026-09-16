package scheduler

import "time"

// Clock interface allows mocking time in tests.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using time.Now
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
