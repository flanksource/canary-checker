package api

import (
	"testing"
	"time"
)

func Test_getMostSuitableWindowDuration(t *testing.T) {
	day := time.Hour * 24

	tests := []struct {
		schedule      time.Duration // how often the check is run
		rangeDuration time.Duration
		expected      time.Duration // the best duration to partition the range
	}{
		{time.Second * 30, time.Minute * 5, 0},
		{time.Second * 30, time.Minute * 30, 0},
		{time.Second * 30, time.Hour * 2, time.Minute},
		{time.Second * 30, time.Hour * 12, time.Minute * 5},
		{time.Second * 30, day * 2, time.Minute * 30},
		{time.Hour, day * 4, 0},
		{time.Hour, day * 5, 0},
		{time.Hour, day * 6, 0},
		{time.Hour, day * 12, time.Hour * 3},
		{time.Second * 30, day * 8, time.Hour * 3},
		{time.Second * 30, day * 30, time.Hour * 6},
		{time.Second * 30, day * 90, day},
		{time.Second * 30, day * 365, day * 7},
	}

	for _, td := range tests {
		t.Run(td.rangeDuration.String(), func(t *testing.T) {
			totalChecks := int(td.rangeDuration / td.schedule)
			result := GetBestPartitioner(totalChecks, td.rangeDuration)
			if result != td.expected {
				t.Errorf("expected %v, but got %v", td.expected, result)
			}
		})
	}
}
