package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasklog/internal/config"
)

// TestUpdateExistingConfig_Integration tests the migration flow end-to-end
func TestUpdateExistingConfig_Integration(t *testing.T) {
	tests := []struct {
		name                   string
		initialConfig          string
		expectNeedsUpdate      bool
		expectDeprecatedFields []string
		expectMissingFields    []string
		shouldNotContain       []string
	}{
		{
			name: "adds task_statuses to v0 config with user_token",
			initialConfig: `jira:
  url: "https://example.atlassian.net"
  username: "user@example.com"
  api_token: "token123"
  project_key: "PROJ"

slack:
  user_token: "xoxp-valid-token"
  channel_id: "C123"

database:
  path: ""
`,
			expectNeedsUpdate:   true,
			expectMissingFields: []string{"jira.task_statuses"},
		},
		{
			name: "v0 config with root-level shortcuts/breaks migrates to v1 nested structure",
			initialConfig: `update:
  disabled: false
  check_interval: "24h"
jira:
  url: "https://example.atlassian.net"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
tempo:
  enabled: false
  api_token: ""
labels:
  allowed_labels:
    - development
shortcuts:
  - name: daily
    task: PROJ-123
    time: 30m
    label: meeting
database:
  path: ""
slack:
  user_token: "xoxp-token"
  channel_id: "C123"
breaks:
  - name: lunch
    duration: 60
    emoji: ":fork_and_knife:"
`,
			expectNeedsUpdate: true, // v0 needs migration to v1 (moves shortcuts/breaks to nested)
		},
		{
			name: "adds missing task_statuses only",
			initialConfig: `jira:
  url: "https://example.atlassian.net"
  project_key: "PROJ"

slack:
  user_token: "xoxp-token"
  channel_id: "C123"
`,
			expectNeedsUpdate:   true,
			expectMissingFields: []string{"jira.task_statuses"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			err := os.WriteFile(configPath, []byte(tt.initialConfig), 0600)
			if err != nil {
				t.Fatalf("failed to create test config: %v", err)
			}

			// Read and migrate config
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("failed to read test config: %v", err)
			}

			updatedData, summary, err := config.MigrateConfig(data)
			if err != nil {
				t.Fatalf("migration failed: %v", err)
			}

			// Verify summary
			if summary.NeedsUpdate != tt.expectNeedsUpdate {
				t.Errorf("expected NeedsUpdate=%v, got %v", tt.expectNeedsUpdate, summary.NeedsUpdate)
			}

			if len(tt.expectDeprecatedFields) > 0 {
				for _, expected := range tt.expectDeprecatedFields {
					found := false
					for _, actual := range summary.DeprecatedFields {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected deprecated field %q not found in %v", expected, summary.DeprecatedFields)
					}
				}
			}

			if len(tt.expectMissingFields) > 0 {
				for _, expected := range tt.expectMissingFields {
					found := false
					for _, actual := range summary.MissingFields {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected missing field %q not found in %v", expected, summary.MissingFields)
					}
				}
			}

			// If update was needed, verify the migration
			if tt.expectNeedsUpdate {
				resultStr := string(updatedData)

				for _, shouldNotContain := range tt.shouldNotContain {
					if strings.Contains(resultStr, shouldNotContain) {
						t.Errorf("expected result NOT to contain %q, but it did", shouldNotContain)
					}
				}
			}
		})
	}
}

func TestUpdateExistingConfig_PreservesValues(t *testing.T) {
	initialConfig := `jira:
  url: "https://my-domain.atlassian.net"
  username: "myuser@example.com"
  api_token: "my-secret-token"
  project_key: "MYPROJ"

slack:
  user_token: "xoxp-preserved"
  channel_id: "C987654"

database:
  path: "/custom/path"
`

	updatedData, _, err := config.MigrateConfig([]byte(initialConfig))
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	resultStr := string(updatedData)

	// Verify original values are preserved
	expectedPreserved := []string{
		"my-domain.atlassian.net",
		"myuser@example.com",
		"my-secret-token",
		"MYPROJ",
		"C987654",
		"/custom/path",
		"xoxp-preserved", // user_token should be preserved
	}

	for _, value := range expectedPreserved {
		if !strings.Contains(resultStr, value) {
			t.Errorf("expected original value %q to be preserved", value)
		}
	}
}

func TestConfirmUpdate(t *testing.T) {
	// Note: confirmUpdate reads from os.Stdin, so we can't easily unit test it
	// This is a placeholder to show where such tests would go if we refactored
	// to accept an io.Reader parameter

	t.Run("function exists and is callable", func(t *testing.T) {
		// Just verify the function signature is correct
		// In a real scenario, we'd inject the reader and test various inputs
		_ = confirmUpdate
	})
}
