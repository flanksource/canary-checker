package utils

import (
	"testing"
	"time"

	"github.com/samber/lo"
)

func TestParseTime(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *time.Time
	}{
		{
			name:     "RFC3339",
			input:    "2023-04-05T15:04:05Z",
			expected: lo.ToPtr(time.Date(2023, 4, 5, 15, 4, 5, 0, time.UTC)),
		},
		{
			name:     "RFC3339Nano",
			input:    "2023-04-05T15:04:05.999999999Z",
			expected: lo.ToPtr(time.Date(2023, 4, 5, 15, 4, 5, 999999999, time.UTC)),
		},
		{
			name:     "ISO8601 with timezone",
			input:    "2023-04-05T15:04:05+02:00",
			expected: lo.ToPtr(time.Date(2023, 4, 5, 15, 4, 5, 0, time.FixedZone("", 2*60*60))),
		},
		{
			name:     "ISO8601 without timezone",
			input:    "2023-04-05T15:04:05",
			expected: lo.ToPtr(time.Date(2023, 4, 5, 15, 4, 5, 0, time.UTC)),
		},
		{
			name:     "MySQL datetime format",
			input:    "2023-04-05 15:04:05",
			expected: lo.ToPtr(time.Date(2023, 4, 5, 15, 4, 5, 0, time.UTC)),
		},
		{
			name:     "Invalid format",
			input:    "not a valid time",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseTime(tc.input)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, but got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected %v, but got nil", tc.expected)
				} else if !result.Equal(*tc.expected) {
					t.Errorf("Expected %v, but got %v", tc.expected, result)
				}
			}
		})
	}
}
