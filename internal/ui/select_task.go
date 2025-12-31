package ui

import (
	"fmt"

	"tasklog/internal/jira"

	"github.com/charmbracelet/huh"
)

// SelectTask presents the user with task selection options
func SelectTask(inProgressIssues []jira.Issue) (*jira.Issue, error) {
	if len(inProgressIssues) == 0 {
		return selectTaskWithoutInProgress()
	}

	searchOption := "Search for a task"
	manualOption := "Enter task key manually"

	options := make([]huh.Option[string], 0, len(inProgressIssues)+2)
	for _, issue := range inProgressIssues {
		val := fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary)
		options = append(options, huh.NewOption(val, val))
	}
	options = append(options,
		huh.NewOption(searchOption, searchOption),
		huh.NewOption(manualOption, manualOption),
	)

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a task").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	if selected == searchOption {
		return promptTaskSearch()
	}
	if selected == manualOption {
		return promptManualTaskKey()
	}

	for _, issue := range inProgressIssues {
		if fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary) == selected {
			return &issue, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}

func selectTaskWithoutInProgress() (*jira.Issue, error) {
	searchOption := "Search for a task"
	manualOption := "Enter task key manually"

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("No in-progress tasks found. How would you like to find a task?").
				Options(
					huh.NewOption(searchOption, searchOption),
					huh.NewOption(manualOption, manualOption),
				).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	if selected == searchOption {
		return promptTaskSearch()
	}
	return promptManualTaskKey()
}

func promptTaskSearch() (*jira.Issue, error) {
	var searchKey string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter task key to search").
				Value(&searchKey).
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("task key is required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return &jira.Issue{Key: searchKey}, nil
}

func promptManualTaskKey() (*jira.Issue, error) {
	var taskKey string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter task key").
				Value(&taskKey).
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("task key is required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return &jira.Issue{Key: taskKey}, nil
}

// SelectFromSearchResults presents search results to the user
func SelectFromSearchResults(issues []jira.Issue) (*jira.Issue, error) {
	if len(issues) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}

	options := make([]huh.Option[string], len(issues))
	for i, issue := range issues {
		val := fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary)
		options[i] = huh.NewOption(val, val)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a task from search results").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, issue := range issues {
		if fmt.Sprintf("%s - %s", issue.Key, issue.Fields.Summary) == selected {
			return &issue, nil
		}
	}

	return nil, fmt.Errorf("task not found")
}
