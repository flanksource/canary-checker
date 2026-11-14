package checks

import "testing"

func TestPacketLossExceeds(t *testing.T) {
	tests := []struct {
		name      string
		loss      float64
		threshold int64
		want      bool
	}{
		{"loss below threshold passes", 5, 10, false},
		{"loss equal to threshold passes", 10, 10, false},
		{"loss above threshold fails", 20, 10, true},
		{"no loss with zero threshold passes", 0, 0, false},
		{"any loss with zero threshold fails", 0.5, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packetLossExceeds(tt.loss, tt.threshold); got != tt.want {
				t.Fatalf("packetLossExceeds(%v, %d) = %v, want %v", tt.loss, tt.threshold, got, tt.want)
			}
		})
	}
}
