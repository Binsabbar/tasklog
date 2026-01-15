package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatTempoTime(t *testing.T) {
	// Save original local time
	origLocal := time.Local
	defer func() { time.Local = origLocal }()

	// Set local time to UTC for testing
	utc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal("Failed to load UTC location")
	}
	time.Local = utc

	tests := []struct {
		name         string
		startDate    string
		startTime    string
		userTimeZone string
		expected     string
	}{
		{
			name:         "Convert KSA to UTC",
			startDate:    "2025-01-05",
			startTime:    "11:00:00",
			userTimeZone: "Asia/Riyadh", // UTC+3
			expected:     "08:00:00",    // 11:00 - 3h = 08:00
		},
		{
			name:         "Same Timezone",
			startDate:    "2025-01-05",
			startTime:    "09:00:00",
			userTimeZone: "UTC",
			expected:     "09:00:00",
		},
		{
			name:         "Empty Timezone",
			startDate:    "2025-01-05",
			startTime:    "09:00:00",
			userTimeZone: "",
			expected:     "09:00:00",
		},
		{
			name:         "Invalid Timezone",
			startDate:    "2025-01-05",
			startTime:    "09:00:00",
			userTimeZone: "Mars/Crater",
			expected:     "09:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTempoTime(tt.startDate, tt.startTime, tt.userTimeZone)
			assert.Equal(t, tt.expected, result)
		})
	}
}
