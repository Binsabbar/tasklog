package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// SelectLabel prompts the user to select a label
func SelectLabel(allowedLabels []string) (string, error) {
	if len(allowedLabels) == 0 {
		return promptFreeTextLabel()
	}

	options := make([]huh.Option[string], len(allowedLabels))
	for i, l := range allowedLabels {
		options[i] = huh.NewOption(l, l)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a label").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}

func promptFreeTextLabel() (string, error) {
	var label string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter a label").
				Value(&label).
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("label is required")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return label, nil
}
