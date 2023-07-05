package api

import (
	"testing"
	"time"
)

func Test_getMostSuitableWindowDuration(t *testing.T) {
	day := time.Hour * 24

	tests := []struct {
		rangeDuration time.Duration
		expected      time.Duration
	}{
		{time.Minute * 5, time.Minute},
		{time.Minute * 30, time.Minute},
		{time.Hour * 2, time.Minute},
		{time.Hour * 12, time.Minute * 5},
		{day * 2, time.Minute * 30},
		{day * 8, time.Hour * 3},
		{day * 30, time.Hour * 6},
		{day * 90, day},
		{day * 365, day * 7},
	}

	for _, test := range tests {
		t.Run(test.rangeDuration.String(), func(t *testing.T) {
			result := getBestPartitioner(test.rangeDuration)
			if result != test.expected {
				t.Errorf("expected %v, but got %v", test.expected, result)
			}
		})
	}
}
