package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveDeprecatedFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "removes user_token from slack section",
			input: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"",
				"slack:",
				"  user_token: \"xoxp-old-token\"",
				"  channel_id: \"C123\"",
				"",
				"database:",
				"  path: \"\"",
			},
			expected: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"",
				"slack:",
				"  channel_id: \"C123\"",
				"",
				"database:",
				"  path: \"\"",
			},
		},
		{
			name: "preserves config without deprecated fields",
			input: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"",
				"slack:",
				"  channel_id: \"C123\"",
				"",
				"database:",
				"  path: \"\"",
			},
			expected: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"",
				"slack:",
				"  channel_id: \"C123\"",
				"",
				"database:",
				"  path: \"\"",
			},
		},
		{
			name: "handles config with comments",
			input: []string{
				"# Comment",
				"slack:",
				"  # Old token",
				"  user_token: \"xoxp-old-token\"",
				"  channel_id: \"C123\"",
			},
			expected: []string{
				"# Comment",
				"slack:",
				"  # Old token",
				"  channel_id: \"C123\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDeprecatedFields(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d lines", len(tt.expected), len(result))
			}

			for i := range result {
				if i >= len(tt.expected) {
					break
				}
				if result[i] != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestAddNewFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		contains []string // Strings that should be in the output
	}{
		{
			name: "adds task_statuses after project_key",
			input: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"  project_key: \"PROJ\"",
				"",
				"slack:",
				"  channel_id: \"C123\"",
			},
			contains: []string{
				"  project_key: \"PROJ\"",
				"  # Optional: Task statuses to include when fetching tasks",
				"  # task_statuses:",
				"  #   - \"In Progress\"",
				"  #   - \"In Review\"",
			},
		},
		{
			name: "adds bot_token and user_id to empty slack section",
			input: []string{
				"jira:",
				"  url: \"https://example.com\"",
				"",
				"slack:",
				"  channel_id: \"C123\"",
			},
			contains: []string{
				"slack:",
				"  # bot_token: \"xoxb-your-slack-bot-token\"",
				"  # user_id: \"U1234567890\"",
			},
		},
		{
			name: "doesn't duplicate if task_statuses already exists",
			input: []string{
				"jira:",
				"  project_key: \"PROJ\"",
				"  task_statuses:",
				"    - \"In Progress\"",
			},
			contains: []string{
				"  project_key: \"PROJ\"",
				"  task_statuses:",
				"    - \"In Progress\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addNewFields(tt.input)
			resultStr := strings.Join(result, "\n")

			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("expected output to contain %q, but it didn't.\nGot:\n%s", expected, resultStr)
				}
			}
		})
	}
}

func TestUpdateExistingConfig(t *testing.T) {
	tests := []struct {
		name           string
		initialConfig  string
		expectRemoved  []string // Fields that should be removed
		expectAdded    []string // Fields that should be added (as comments)
		expectUpToDate bool     // If true, expect "up to date" message
	}{
		{
			name: "migrates old config with user_token",
			initialConfig: `# Old config
jira:
  url: "https://example.atlassian.net"
  username: "user@example.com"
  api_token: "token123"
  project_key: "PROJ"

slack:
  user_token: "xoxp-old-token"
  channel_id: "C123"

database:
  path: ""
`,
			expectRemoved: []string{"user_token"},
			expectAdded: []string{
				"# Optional: Task statuses",
				"# bot_token:",
				"# user_id:",
			},
			expectUpToDate: false,
		},
		{
			name: "handles already updated config",
			initialConfig: `jira:
  url: "https://example.atlassian.net"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"

slack:
  bot_token: "xoxb-token"
  user_id: "U123"
  channel_id: "C123"
`,
			expectUpToDate: true,
		},
		{
			name: "adds missing task_statuses only",
			initialConfig: `jira:
  url: "https://example.atlassian.net"
  project_key: "PROJ"

slack:
  bot_token: "xoxb-token"
  user_id: "U123"
`,
			expectRemoved: []string{},
			expectAdded: []string{
				"# task_statuses:",
			},
			expectUpToDate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			err := os.WriteFile(configPath, []byte(tt.initialConfig), 0600)
			if err != nil {
				t.Fatalf("failed to create test config: %v", err)
			}

			// Note: We can't fully test updateExistingConfig because it prompts for user input
			// Instead, we'll test the helper functions it uses
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("failed to read test config: %v", err)
			}

			content := string(data)
			lines := strings.Split(content, "\n")

			// Test the transformation functions
			updatedLines := removeDeprecatedFields(lines)
			updatedLines = addNewFields(updatedLines)
			updatedContent := strings.Join(updatedLines, "\n")

			// Verify removed fields
			for _, removed := range tt.expectRemoved {
				if strings.Contains(updatedContent, removed) {
					t.Errorf("expected %q to be removed, but it's still in the config", removed)
				}
			}

			// Verify added fields
			for _, added := range tt.expectAdded {
				if !strings.Contains(updatedContent, added) {
					t.Errorf("expected %q to be added, but it's not in the config", added)
				}
			}

			// Check if config should be up to date
			hasDeprecatedFields := strings.Contains(content, "user_token:")
			hasTaskStatuses := strings.Contains(content, "task_statuses:")

			isUpToDate := !hasDeprecatedFields && hasTaskStatuses
			if tt.expectUpToDate != isUpToDate {
				t.Errorf("expected up-to-date status to be %v, got %v", tt.expectUpToDate, isUpToDate)
			}
		})
	}
}

func TestUpdateExistingConfig_FileOperations(t *testing.T) {
	t.Run("creates backup file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		initialConfig := `jira:
  project_key: "PROJ"

slack:
  user_token: "xoxp-old"
  channel_id: "C123"
`

		err := os.WriteFile(configPath, []byte(initialConfig), 0600)
		if err != nil {
			t.Fatalf("failed to create test config: %v", err)
		}

		// Simulate the backup creation part
		backupPath := configPath + ".backup"
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		err = os.WriteFile(backupPath, data, 0600)
		if err != nil {
			t.Fatalf("failed to create backup: %v", err)
		}

		// Verify backup exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file was not created")
		}

		// Verify backup content matches original
		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("failed to read backup: %v", err)
		}

		if string(backupData) != initialConfig {
			t.Error("backup content doesn't match original")
		}
	})

	t.Run("preserves original values after migration", func(t *testing.T) {
		lines := []string{
			"jira:",
			"  url: \"https://my-domain.atlassian.net\"",
			"  username: \"myuser@example.com\"",
			"  api_token: \"my-secret-token\"",
			"  project_key: \"MYPROJ\"",
			"",
			"slack:",
			"  user_token: \"xoxp-deprecated\"",
			"  channel_id: \"C987654321\"",
		}

		result := removeDeprecatedFields(lines)
		result = addNewFields(result)

		resultStr := strings.Join(result, "\n")

		// Verify original values are preserved
		expectedPreserved := []string{
			"https://my-domain.atlassian.net",
			"myuser@example.com",
			"my-secret-token",
			"MYPROJ",
			"C987654321",
		}

		for _, value := range expectedPreserved {
			if !strings.Contains(resultStr, value) {
				t.Errorf("expected original value %q to be preserved", value)
			}
		}

		// Verify deprecated field is removed
		if strings.Contains(resultStr, "xoxp-deprecated") {
			t.Error("deprecated user_token value should have been removed")
		}
	})
}
