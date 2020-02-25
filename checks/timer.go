package checks

import "time"

type Timer struct {
	Start time.Time
}

func (t Timer) Elapsed() float64 {
	return float64(time.Since(t.Start).Milliseconds())
}

func NewTimer() Timer {
	return Timer{Start: time.Now()}
}
