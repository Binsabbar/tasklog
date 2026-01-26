package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands a path that may begin with '~' to the user's home directory.
// Paths not starting with '~' are returned unchanged.
func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	// Handle "~" alone or "~/"
	if path == "~" {
		return home, nil
	}

	// Replace "~/" with home directory
	return filepath.Join(home, path[2:]), nil
}

// ensureDirForFile ensures that the parent directory of the given file path exists.
// Creates all necessary parent directories with 0700 permissions.
func ensureDirForFile(path string) error {
	dir := filepath.Dir(path)

	// If dir is "." (current directory), no need to create anything
	if dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	return nil
}
