package ui

import (
	"fmt"
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
		return promptTaskSearch()
	}
	if selected == "Enter task key manually" {
		return promptManualTaskKey()
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
		return promptTaskSearch()
	}
	return promptManualTaskKey()
}

// promptTaskSearch prompts the user to search for a task
func promptTaskSearch() (*jira.Issue, error) {
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

// promptManualTaskKey prompts the user to enter a task key manually
func promptManualTaskKey() (*jira.Issue, error) {
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

// PromptTimeSpent prompts the user for time spent
func PromptTimeSpent() (string, error) {
	var timeSpent string
	prompt := &survey.Input{
		Message: "Enter time spent (e.g., 2h 30m, 2.5h, 150m):",
		Help:    "Formats: 2h 30m, 2.5h, 150m (will be rounded to nearest 5 minutes)",
	}

	if err := survey.AskOne(prompt, &timeSpent, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}

	return timeSpent, nil
}

// SelectLabel prompts the user to select a label
func SelectLabel(allowedLabels []string) (string, error) {
	if len(allowedLabels) == 0 {
		// If no labels configured, allow free text
		return promptFreeTextLabel()
	}

	var selected string
	prompt := &survey.Select{
		Message:  "Select a label:",
		Options:  allowedLabels,
		PageSize: 10,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	return selected, nil
}

// promptFreeTextLabel prompts for a free-text label
func promptFreeTextLabel() (string, error) {
	var label string
	prompt := &survey.Input{
		Message: "Enter a label:",
	}

	if err := survey.AskOne(prompt, &label, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}

	return label, nil
}

// PromptComment prompts the user for an optional comment
func PromptComment() (string, error) {
	var comment string
	prompt := &survey.Input{
		Message: "Enter a comment (optional):",
	}

	if err := survey.AskOne(prompt, &comment); err != nil {
		return "", err
	}

	return comment, nil
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

// PromptStartTime prompts user for when they worked (optional)
// Returns time.Now() if user wants current time, otherwise parsed datetime
func PromptStartTime() (time.Time, error) {
	useNow, err := Confirm("Log for current time?")
	if err != nil {
		return time.Time{}, err
	}

	if useNow {
		return time.Now(), nil
	}

	var whenStr string
	prompt := &survey.Input{
		Message: "When did you work on this?",
		Help:    "Examples: 2pm, yesterday 3pm, 2 hours ago, 14:30",
	}

	if err := survey.AskOne(prompt, &whenStr, survey.WithValidator(survey.Required)); err != nil {
		return time.Time{}, err
	}

	// We need to import timeparse, but can't introduce import cycle if UI is imported by timeparse.
	// However, looking at imports, cmd imports ui and timeparse. ui doesn't import timeparse.
	// Oh wait, PromptStartTime needs to call timeparse.ParseDateTime.
	// Check imports in internal/ui/prompts.go.
	// It imports "tasklog/internal/jira".
	// Make sure timeparse is not importing ui.
	// internal/timeparse/datetime.go imports "github.com/olebedev/when" etc. No internal imports.

	// So we can import timeparse here.
	return timeparse.ParseDateTime(whenStr)
}
