package checks

import (
	"testing"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

func TestGetNextRuntime(t *testing.T) {
	lastRuntime := time.Date(2020, time.January, 2, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name     string
		schedule string
		interval uint64
	}{
		{name: "cron", schedule: "*/15 * * * *"},
		{name: "every", schedule: "@every 15m"},
		{name: "interval", interval: 900},
	} {
		t.Run(tt.name, func(t *testing.T) {
			canary := v1.Canary{}
			canary.Spec.Schedule = tt.schedule
			canary.Spec.Interval = tt.interval
			got, err := getNextRuntime(canary, lastRuntime)
			if err != nil {
				t.Fatal(err)
			}
			want := lastRuntime.Add(15 * time.Minute)
			if got == nil || !got.Equal(want) {
				t.Fatalf("getNextRuntime() = %v, want %v", got, want)
			}
		})
	}
}

func TestGetNextRuntimeInvalidSchedule(t *testing.T) {
	canary := v1.Canary{}
	canary.Spec.Schedule = "invalid"
	got, err := getNextRuntime(canary, time.Now())
	if err == nil || got != nil {
		t.Fatalf("getNextRuntime() = %v, %v; want nil and an error", got, err)
	}
}
