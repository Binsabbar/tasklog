package ui

import (
	"fmt"
	"strings"
	"time"

	"tasklog/internal/jira"
	"tasklog/internal/timeparse"

	"github.com/AlecAivazis/survey/v2"
)

// SelectTask presents the user with task selection options
func SelectTask(inProgressIssues []jira.Issue) (*jira.Issue, error) {
	if len(inProgressIssues) == 0 {
		// No in-progress tasks, prompt for search or manual entry
		return selectTaskWithoutInProgress()
	}

	// Build options from in-progress tasks
	options := make([]string, 0, len(inProgressIssues)+2)
	for _, issue := range inProgressIssues {
		options = append(options, fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary))
	}
	options = append(options, "Search for a task", "Enter task key manually")

	var selected string
	prompt := &survey.Select{
		Message:  "Select a task:",
		Options:  options,
		PageSize: 10,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	// Check if user selected search or manual entry
	if selected == "Search for a task" {
		return PromptTaskSearch()
	}
	if selected == "Enter task key manually" {
		return PromptManualTaskKey()
	}

	// Find the selected issue
	for _, issue := range inProgressIssues {
		if fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary) == selected {
			return &issue, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}

// selectTaskWithoutInProgress handles task selection when no in-progress tasks exist
func selectTaskWithoutInProgress() (*jira.Issue, error) {
	options := []string{"Search for a task", "Enter task key manually"}

	var selected string
	prompt := &survey.Select{
		Message: "No in-progress tasks found. How would you like to find a task?",
		Options: options,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	if selected == "Search for a task" {
		return PromptTaskSearch()
	}
	return PromptManualTaskKey()
}

// PromptTaskSearch prompts the user to search for a task
func PromptTaskSearch() (*jira.Issue, error) {
	var searchKey string
	prompt := &survey.Input{
		Message: "Enter task key to search:",
	}

	if err := survey.AskOne(prompt, &searchKey, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// Return a placeholder - actual search will be done by the caller
	return &jira.Issue{Key: searchKey}, nil
}

// PromptManualTaskKey prompts the user to enter a task key manually
func PromptManualTaskKey() (*jira.Issue, error) {
	var taskKey string
	prompt := &survey.Input{
		Message: "Enter task key:",
	}

	if err := survey.AskOne(prompt, &taskKey, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	return &jira.Issue{Key: taskKey}, nil
}

// SelectFromSearchResults presents search results to the user
func SelectFromSearchResults(issues []jira.Issue) (*jira.Issue, error) {
	if len(issues) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}

	options := make([]string, len(issues))
	for i, issue := range issues {
		options[i] = fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary)
	}

	var selected string
	prompt := &survey.Select{
		Message:  "Select a task from search results:",
		Options:  options,
		PageSize: 10,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	// Find the selected issue
	for _, issue := range issues {
		if fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary) == selected {
			return &issue, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}

// PromptTimeSpent prompts the user for time spent.
// Retries on invalid input until valid time is entered or user cancels with Ctrl+C.
// Returns the time in seconds.
func PromptTimeSpent() (int, error) {
	fmt.Println("(Press Ctrl+C to cancel)")

	for {
		var timeSpent string
		prompt := &survey.Input{
			Message: "Enter time spent (e.g., 2h 30m, 2.5h, 150m):",
			Help:    "Formats: 2h 30m, 2.5h, 150m (will be rounded to nearest 5 minutes)",
		}

		if err := survey.AskOne(prompt, &timeSpent, survey.WithValidator(survey.Required)); err != nil {
			return 0, err
		}

		// Try to parse the time
		timeSeconds, err := timeparse.Parse(timeSpent)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		return timeSeconds, nil
	}
}

// PromptComment prompts the user for a required comment/description
func PromptComment() (string, error) {
	const minLength = 5

	var comment string
	prompt := &survey.Input{
		Message: "Enter a description (required for Tempo):",
	}

	// Custom validator that trims whitespace and checks minimum length
	validator := func(val interface{}) error {
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("invalid input type")
		}

		trimmed := strings.TrimSpace(str)
		if len(trimmed) == 0 {
			return fmt.Errorf("description cannot be empty or contain only whitespace")
		}

		if len(trimmed) < minLength {
			return fmt.Errorf("description must be at least %d characters (currently %d)", minLength, len(trimmed))
		}

		return nil
	}

	if err := survey.AskOne(prompt, &comment, survey.WithValidator(validator)); err != nil {
		return "", err
	}

	// Trim whitespace from the final comment
	return strings.TrimSpace(comment), nil
}

// Confirm asks the user for confirmation
func Confirm(message string) (bool, error) {
	var confirmed bool
	prompt := &survey.Confirm{
		Message: message,
		Default: true,
	}

	if err := survey.AskOne(prompt, &confirmed); err != nil {
		return false, err
	}

	return confirmed, nil
}

// PromptStartTime prompts user for when they worked (optional).
// Returns time.Now() minus the time spent if user wants current time.
// Retries on invalid input until valid time is entered or user cancels with Ctrl+C.
func PromptStartTime(timeSpentSeconds int) (time.Time, error) {
	useNow, err := Confirm("Log for current time?")
	if err != nil {
		return time.Time{}, err
	}

	if useNow {
		return time.Now().Add(-time.Duration(timeSpentSeconds) * time.Second), nil
	}

	fmt.Println("(Press Ctrl+C to cancel)")

	for {
		var whenStr string
		prompt := &survey.Input{
			Message: "When did you work on this?",
			Help:    "Examples: 2pm, yesterday 3pm, 2 hours ago, 14:30",
		}

		if err := survey.AskOne(prompt, &whenStr, survey.WithValidator(survey.Required)); err != nil {
			return time.Time{}, err
		}

		// Try to parse the datetime
		result, err := timeparse.ParseDateTime(whenStr)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		return result, nil
	}
}
