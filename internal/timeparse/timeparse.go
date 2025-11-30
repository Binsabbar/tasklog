package timeparse

import (
	"fmt"
	"math"
	"strings"

	str2duration "github.com/xhit/go-str2duration/v2"
)

// Parse parses a time string and returns the duration in seconds
// Supports various formats using go-str2duration library
// Rounds to the nearest 5 minutes
func Parse(input string) (int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, fmt.Errorf("empty time input")
	}

	// Normalize the input:
	// 1. Convert to lowercase for case insensitivity
	// 2. Replace full words with abbreviations
	// 3. Remove spaces between numbers and units
	normalized := strings.ToLower(input)
	normalized = strings.ReplaceAll(normalized, "hours", "h")
	normalized = strings.ReplaceAll(normalized, "hour", "h")
	normalized = strings.ReplaceAll(normalized, "minutes", "m")
	normalized = strings.ReplaceAll(normalized, "minute", "m")
	normalized = strings.ReplaceAll(normalized, "mins", "m")
	normalized = strings.ReplaceAll(normalized, "min", "m")
	normalized = strings.ReplaceAll(normalized, " ", "")

	// Parse using the library
	duration, err := str2duration.ParseDuration(normalized)
	if err != nil {
		return 0, fmt.Errorf("invalid time format: %s (expected formats: 2h 30m, 2.5h, 150m, 2h30m)", input)
	}

	if duration <= 0 {
		return 0, fmt.Errorf("time must be positive")
	}

	// Convert to minutes for rounding
	totalMinutes := duration.Minutes()

	// Round to nearest 5 minutes
	roundedMinutes := roundToNearest5(totalMinutes)

	return int(roundedMinutes * 60), nil
}

// roundToNearest5 rounds a number to the nearest 5
func roundToNearest5(minutes float64) float64 {
	return math.Round(minutes/5) * 5
}

// Format formats seconds into a human-readable time string
func Format(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

// Validate checks if a time string is valid
func Validate(input string) error {
	_, err := Parse(input)
	return err
}
