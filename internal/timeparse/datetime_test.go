package timeparse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		checkTime func(t *testing.T, result time.Time)
	}{
		{
			name:    "today specific time",
			input:   "1am",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				// We expect 1am today or yesterday depending on when this runs.
				// But mostly today 1am is past.
				assert.Equal(t, 1, result.Hour())
			},
		},
		{
			name:    "yesterday",
			input:   "yesterday",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				now := time.Now()
				yesterday := now.AddDate(0, 0, -1)
				assert.Equal(t, yesterday.Year(), result.Year())
				assert.Equal(t, yesterday.Month(), result.Month())
				assert.Equal(t, yesterday.Day(), result.Day())
			},
		},
		{
			name:    "hours ago",
			input:   "2 hours ago",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				now := time.Now()
				// Allow slight difference for execution time
				diff := now.Sub(result)
				assert.InDelta(t, 2*time.Hour.Seconds(), diff.Seconds(), 1.0)
			},
		},
		{
			name:      "future time error",
			input:     "tomorrow",
			wantErr:   true,
			checkTime: nil,
		},
		{
			name:      "invalid format",
			input:     "not a time",
			wantErr:   true,
			checkTime: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkTime != nil {
					tt.checkTime(t, got)
				}
			}
		})
	}
}
