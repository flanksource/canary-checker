package utils

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
)

func Age(d time.Duration) string {
	if d.Milliseconds() < 1000 {
		return fmt.Sprintf("%0.dms", d.Milliseconds())
	}
	return duration.HumanDuration(d)
}
