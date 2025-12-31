package ui

import (
	"fmt"
	"time"

	"tasklog/internal/timeparse"

	"github.com/charmbracelet/huh"
)

// PromptTimeSpent prompts the user for time spent
func PromptTimeSpent() (string, error) {
	var timeSpent string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter time spent").
				Description("Formats: 2h 30m, 2.5h, 150m (will be rounded to nearest 5 minutes)").
				Value(&timeSpent).
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("time spent is required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return timeSpent, nil
}

// PromptComment prompts the user for an optional comment
func PromptComment() (string, error) {
	var comment string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter a comment (optional)").
				Value(&comment),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return comment, nil
}

// PromptStartTime prompts user for when they worked
func PromptStartTime() (time.Time, error) {
	useNow, err := Confirm("Log for current time?")
	if err != nil {
		return time.Time{}, err
	}

	if useNow {
		return time.Now(), nil
	}

	var whenStr string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("When did you work on this?").
				Description("Examples: 2pm, yesterday 3pm, 2 hours ago, 14:30").
				Value(&whenStr).
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("time is required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return time.Time{}, err
	}

	return timeparse.ParseDateTime(whenStr)
}
