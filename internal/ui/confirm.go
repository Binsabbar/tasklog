package ui

import (
	"github.com/charmbracelet/huh"
)

// Confirm asks the user for confirmation
func Confirm(message string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirmed),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirmed, nil
}
