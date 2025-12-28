package timeparse

import (
	"fmt"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

// ParseDateTime parses natural language datetime expressions
// Supports: "2pm", "yesterday", "yesterday at 3pm", "2 hours ago", etc.
// Returns error if parsed time is in the future
func ParseDateTime(input string) (time.Time, error) {
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)

	parsed, err := w.Parse(input, time.Now())
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse datetime: %w", err)
	}

	if parsed == nil {
		return time.Time{}, fmt.Errorf("could not understand time expression: %q", input)
	}

	result := parsed.Time

	// Validate not in the future
	if result.After(time.Now()) {
		return time.Time{}, fmt.Errorf("cannot log time in the future: %s", result.Format("Mon Jan 2 15:04"))
	}

	return result, nil
}
