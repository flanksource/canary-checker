package checks

import (
	"fmt"
	"time"
)

type Timer struct {
	Start time.Time
}

func (t Timer) Elapsed() float64 {
	return float64(time.Since(t.Start).Milliseconds())
}

func (t Timer) Millis() int64 {
	return time.Since(t.Start).Milliseconds()
}

func (t Timer) String() string {
	return fmt.Sprintf("%dms", t.Millis())
}

func NewTimer() Timer {
	return Timer{Start: time.Now()}
}
