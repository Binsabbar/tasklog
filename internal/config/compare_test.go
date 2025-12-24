package config

import (
	"strings"
	"testing"
)

func TestCompareWithExample(t *testing.T) {
	tests := []struct {
		name              string
		userConfig        string
		expectUpToDate    bool
		expectMissingKeys []string
		expectExtraKeys   []string
	}{
		{
			name: "up to date config",
			userConfig: `version: 1
jira:
  url: "https://example.com"
  username: "user@example.com"
  api_token: "token"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
  shortcuts: []
tempo:
  enabled: false
  api_token: ""
labels:
  allowed_labels: []
database:
  path: ""
slack:
  user_token: "token"
  channel_id: "C123"
  breaks: []
update:
  disabled: false
  check_interval: "24h"
  channel: ""
`,
			expectUpToDate: true,
		},
		{
			name: "missing optional sections",
			userConfig: `version: 1
jira:
  url: "https://example.com"
  username: "user@example.com"
  api_token: "token"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
  shortcuts: []
tempo:
  enabled: false
  api_token: ""
`,
			expectUpToDate:    false,
			expectMissingKeys: []string{"labels", "database", "slack", "update"},
		},
		{
			name: "missing nested fields",
			userConfig: `version: 1
jira:
  url: "https://example.com"
  username: "user@example.com"
  api_token: "token"
  project_key: "PROJ"
tempo:
  enabled: false
  api_token: ""
labels:
  allowed_labels: []
database:
  path: ""
slack:
  user_token: "token"
  channel_id: "C123"
update:
  disabled: false
  check_interval: "24h"
`,
			expectUpToDate:    false,
			expectMissingKeys: []string{"jira.task_statuses", "jira.shortcuts", "slack.breaks", "update.channel"},
		},
		{
			name: "extra deprecated fields",
			userConfig: `version: 1
jira:
  url: "https://example.com"
  username: "user@example.com"
  api_token: "token"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
  shortcuts: []
tempo:
  enabled: false
  api_token: ""
labels:
  allowed_labels: []
database:
  path: ""
slack:
  user_token: "token"
  channel_id: "C123"
  breaks: []
update:
  disabled: false
  check_interval: "24h"
  channel: ""
old_field: "deprecated"
shortcuts:
  - name: "test"
`,
			expectUpToDate:  true,
			expectExtraKeys: []string{"old_field", "shortcuts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareWithExample([]byte(tt.userConfig))
			if err != nil {
				t.Fatalf("CompareWithExample failed: %v", err)
			}

			if result.IsUpToDate != tt.expectUpToDate {
				t.Errorf("expected IsUpToDate=%v, got %v", tt.expectUpToDate, result.IsUpToDate)
			}

			if len(tt.expectMissingKeys) > 0 {
				if len(result.MissingKeys) != len(tt.expectMissingKeys) {
					t.Errorf("expected %d missing keys, got %d: %v",
						len(tt.expectMissingKeys), len(result.MissingKeys), result.MissingKeys)
				}
				for _, expected := range tt.expectMissingKeys {
					found := false
					for _, actual := range result.MissingKeys {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected missing key %q not found in %v", expected, result.MissingKeys)
					}
				}
			}

			if len(tt.expectExtraKeys) > 0 {
				for _, expected := range tt.expectExtraKeys {
					found := false
					for _, actual := range result.ExtraKeys {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected extra key %q not found in %v", expected, result.ExtraKeys)
					}
				}
			}
		})
	}
}

func TestFormatComparisonResult(t *testing.T) {
	tests := []struct {
		name           string
		result         *ComparisonResult
		expectContains []string
	}{
		{
			name: "up to date",
			result: &ComparisonResult{
				IsUpToDate:  true,
				MissingKeys: []string{},
				ExtraKeys:   []string{},
			},
			expectContains: []string{"✓", "up to date"},
		},
		{
			name: "missing fields",
			result: &ComparisonResult{
				IsUpToDate:  false,
				MissingKeys: []string{"jira.shortcuts", "slack.breaks"},
				ExtraKeys:   []string{},
			},
			expectContains: []string{"Missing fields", "jira.shortcuts", "slack.breaks", "config example"},
		},
		{
			name: "extra fields",
			result: &ComparisonResult{
				IsUpToDate:  true,
				MissingKeys: []string{},
				ExtraKeys:   []string{"old_field", "deprecated_section"},
			},
			expectContains: []string{"Extra fields", "old_field", "deprecated_section", "custom"},
		},
		{
			name: "both missing and extra",
			result: &ComparisonResult{
				IsUpToDate:  false,
				MissingKeys: []string{"new_field"},
				ExtraKeys:   []string{"old_field"},
			},
			expectContains: []string{"Missing fields", "new_field", "Extra fields", "old_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatComparisonResult(tt.result)

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}
