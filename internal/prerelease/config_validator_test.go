package prerelease

import (
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectIssues  int
		expectFields  []string
		expectNoIssue bool
	}{
		{
			name: "detects check_for_updates field",
			config: `version: 1
jira:
  url: https://example.com
update:
  check_for_updates: true
  check_interval: 24h`,
			expectIssues: 1,
			expectFields: []string{"update.check_for_updates"},
		},
		{
			name: "detects disable_update_check field",
			config: `version: 1
jira:
  url: https://example.com
update:
  disable_update_check: false
  check_interval: 24h`,
			expectIssues: 1,
			expectFields: []string{"update.disable_update_check"},
		},
		{
			name: "detects both deprecated fields",
			config: `version: 1
jira:
  url: https://example.com
update:
  check_for_updates: true
  disable_update_check: false
  check_interval: 24h`,
			expectIssues: 2,
			expectFields: []string{"update.check_for_updates", "update.disable_update_check"},
		},
		{
			name: "no issues with correct field name",
			config: `version: 1
jira:
  url: https://example.com
update:
  disabled: false
  check_interval: 24h`,
			expectNoIssue: true,
		},
		{
			name: "no issues when update section missing",
			config: `version: 1
jira:
  url: https://example.com
  project_key: PROJ`,
			expectNoIssue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := ValidateConfig([]byte(tt.config))
			if err != nil {
				t.Fatalf("ValidateConfig failed: %v", err)
			}

			if tt.expectNoIssue {
				if len(issues) != 0 {
					t.Errorf("expected no issues, got %d: %v", len(issues), issues)
				}
				return
			}

			if len(issues) != tt.expectIssues {
				t.Errorf("expected %d issues, got %d", tt.expectIssues, len(issues))
			}

			// Check that expected fields are in the issues
			for _, expectedField := range tt.expectFields {
				found := false
				for _, issue := range issues {
					if issue.Field == expectedField {
						found = true
						// Verify issue has required fields
						if issue.Issue == "" {
							t.Errorf("issue for field %s has empty Issue", expectedField)
						}
						if issue.Suggestion == "" {
							t.Errorf("issue for field %s has empty Suggestion", expectedField)
						}
						if issue.ReleaseNote == "" {
							t.Errorf("issue for field %s has empty ReleaseNote", expectedField)
						}
						break
					}
				}
				if !found {
					t.Errorf("expected issue for field %s not found", expectedField)
				}
			}
		})
	}
}

func TestFormatIssues(t *testing.T) {
	tests := []struct {
		name           string
		issues         []ConfigIssue
		expectEmpty    bool
		expectContains []string
	}{
		{
			name:        "empty issues returns empty string",
			issues:      []ConfigIssue{},
			expectEmpty: true,
		},
		{
			name: "formats single issue",
			issues: []ConfigIssue{
				{
					Field:       "update.check_for_updates",
					Issue:       "Deprecated field",
					Suggestion:  "Use 'disabled' instead",
					ReleaseNote: "v1.0.0-alpha.2",
				},
			},
			expectContains: []string{
				"Configuration issues detected",
				"update.check_for_updates",
				"Deprecated field",
				"Use 'disabled' instead",
				"tasklog init --update",
			},
		},
		{
			name: "formats multiple issues",
			issues: []ConfigIssue{
				{
					Field:       "update.check_for_updates",
					Issue:       "Deprecated field 1",
					Suggestion:  "Fix 1",
					ReleaseNote: "v1.0.0-alpha.2",
				},
				{
					Field:       "update.disable_update_check",
					Issue:       "Deprecated field 2",
					Suggestion:  "Fix 2",
					ReleaseNote: "v1.0.0-alpha.3",
				},
			},
			expectContains: []string{
				"1. Field:",
				"2. Field:",
				"update.check_for_updates",
				"update.disable_update_check",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatIssues(tt.issues)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string, got: %s", result)
				}
				return
			}

			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}
