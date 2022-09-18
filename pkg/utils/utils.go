package utils

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
)

func Age(d time.Duration) string {
	if d.Milliseconds() == 0 {
		return "0ms"
	}
	if d.Milliseconds() < 1000 {
		return fmt.Sprintf("%0.dms", d.Milliseconds())
	}
	return duration.HumanDuration(d)
}

// SetDifference returns the list of elements present in a but not present in b
func SetDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
