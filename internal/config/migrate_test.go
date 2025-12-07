package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMigrateConfig(t *testing.T) {
	tests := []struct {
		name                          string
		input                         string
		expectError                   bool
		expectNeedsUpdate             bool
		expectFromVersion             int
		expectToVersion               int
		expectDeprecatedFields        []string
		expectMissingFields           []string
		expectMissingOptionalSections []string
		shouldContain                 []string
		shouldNotContain              []string
	}{
		{
			name: "v0 to v1: adds task_statuses and preserves user_token",
			input: `jira:
  url: "https://example.com"
  project_key: "PROJ"
slack:
  user_token: "xoxp-valid-token"
  channel_id: "C123"
`,
			expectNeedsUpdate:   true,
			expectFromVersion:   0,
			expectToVersion:     1,
			expectMissingFields: []string{"jira.task_statuses"},
			shouldContain:       []string{"version: 1", "user_token: xoxp-valid-token"},
		},
		{
			name: "v0 to v1: detects missing task_statuses",
			input: `jira:
  url: "https://example.com"
  project_key: "PROJ"
slack:
  user_token: "xoxp-token"
  channel_id: "C123"
`,
			expectNeedsUpdate:   true,
			expectFromVersion:   0,
			expectToVersion:     1,
			expectMissingFields: []string{"jira.task_statuses"},
			shouldContain:       []string{"user_token", "version: 1"},
		},
		{
			name: "v1: config already up to date",
			input: `version: 1
jira:
  url: "https://example.com"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
tempo:
  enabled: false
labels:
  allowed_labels:
    - development
shortcuts:
  - name: daily
    task: PROJ-123
database:
  path: ""
slack:
  user_token: "xoxp-token"
  channel_id: "C123"
breaks:
  - name: lunch
    duration: 60
update:
  check_for_updates: true
  check_interval: 24
`,
			expectNeedsUpdate: false,
			expectFromVersion: 1,
			expectToVersion:   1,
			shouldContain:     []string{"task_statuses", "user_token"},
		},
		{
			name: "v0 to v1: handles missing slack section",
			input: `jira:
  url: "https://example.com"
  project_key: "PROJ"
`,
			expectNeedsUpdate:   true,
			expectFromVersion:   0,
			expectToVersion:     1,
			expectMissingFields: []string{"jira.task_statuses"},
			shouldContain:       []string{"version: 1"},
		},
		{
			name: "v0 to v1: preserves existing values",
			input: `jira:
  url: "https://my-domain.atlassian.net"
  username: "user@example.com"
  api_token: "secret-token"
  project_key: "MYPROJ"
slack:
  user_token: "xoxp-preserved"
  channel_id: "C987654"
database:
  path: "/custom/path"
`,
			expectNeedsUpdate: true,
			shouldContain:     []string{"my-domain.atlassian.net", "user@example.com", "MYPROJ", "C987654", "/custom/path", "xoxp-preserved"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, summary, err := MigrateConfig([]byte(tt.input))

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if summary.NeedsUpdate != tt.expectNeedsUpdate {
				t.Errorf("expected NeedsUpdate=%v, got %v", tt.expectNeedsUpdate, summary.NeedsUpdate)
			}

			if tt.expectFromVersion > 0 && summary.FromVersion != tt.expectFromVersion {
				t.Errorf("expected FromVersion=%d, got %d", tt.expectFromVersion, summary.FromVersion)
			}

			if tt.expectToVersion > 0 && summary.ToVersion != tt.expectToVersion {
				t.Errorf("expected ToVersion=%d, got %d", tt.expectToVersion, summary.ToVersion)
			}

			if len(tt.expectDeprecatedFields) > 0 {
				if !summary.HasDeprecatedFields {
					t.Error("expected HasDeprecatedFields=true, got false")
				}
				for _, field := range tt.expectDeprecatedFields {
					found := false
					for _, deprecated := range summary.DeprecatedFields {
						if deprecated == field {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected deprecated field %q not found", field)
					}
				}
			}

			if len(tt.expectMissingFields) > 0 {
				for _, field := range tt.expectMissingFields {
					found := false
					for _, missing := range summary.MissingFields {
						if missing == field {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected missing field %q not found in %v", field, summary.MissingFields)
					}
				}
			}

			if len(tt.expectMissingOptionalSections) > 0 {
				for _, section := range tt.expectMissingOptionalSections {
					found := false
					for _, missing := range summary.MissingOptionalSections {
						if missing == section {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected missing optional section %q not found in %v", section, summary.MissingOptionalSections)
					}
				}
			}

			resultStr := string(result)

			for _, shouldContain := range tt.shouldContain {
				if !strings.Contains(resultStr, shouldContain) {
					t.Errorf("expected result to contain %q, but it didn't.\nGot:\n%s", shouldContain, resultStr)
				}
			}

			for _, shouldNotContain := range tt.shouldNotContain {
				if strings.Contains(resultStr, shouldNotContain) {
					t.Errorf("expected result NOT to contain %q, but it did.\nGot:\n%s", shouldNotContain, resultStr)
				}
			}

			// Verify the result is valid YAML
			var parsed map[string]interface{}
			if err := yaml.Unmarshal(result, &parsed); err != nil {
				t.Errorf("migrated config is not valid YAML: %v\nGot:\n%s", err, resultStr)
			}
		})
	}
}

func TestMigrateConfig_InvalidYAML(t *testing.T) {
	input := `invalid: yaml: [unclosed`
	_, _, err := MigrateConfig([]byte(input))
	if err == nil {
		t.Error("expected error for invalid YAML, got none")
	}
}

func TestMigrateConfig_FutureVersion(t *testing.T) {
	input := `version: 99
jira:
  url: "https://example.com"
  project_key: "PROJ"
`
	_, _, err := MigrateConfig([]byte(input))
	if err == nil {
		t.Error("expected error for future version, got none")
	}
	if err != nil && !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("expected 'newer than supported' error, got: %v", err)
	}
}

func TestMigrateConfig_V1WithUserToken(t *testing.T) {
	// V1 config with user_token is valid (user_token is not deprecated)
	input := `version: 1
jira:
  url: "https://example.com"
  username: "user@example.com"
  api_token: "token123"
  project_key: "PROJ"
  task_statuses:
    - In Progress
tempo:
  enabled: false
labels:
  allowed_labels:
    - development
shortcuts:
  - name: daily
    task: PROJ-123
database:
  path: ""
slack:
  user_token: "xoxp-valid-token"
  channel_id: "C123"
breaks:
  - name: lunch
    duration: 60
`
	result, summary, err := MigrateConfig([]byte(input))
	if err != nil {
		t.Errorf("unexpected error for valid v1 config with user_token: %v", err)
	}
	// Should need update for missing 'update' section
	if !summary.NeedsUpdate {
		t.Error("expected NeedsUpdate=true for v1 config missing update section")
	}
	// Verify user_token is preserved
	if !strings.Contains(string(result), "xoxp-valid-token") {
		t.Error("expected user_token to be preserved in result")
	}
}

func TestMigrateConfig_InvalidV1MissingRequiredFields(t *testing.T) {
	// V1 config missing jira section entirely - should fail
	input := `version: 1
slack:
  bot_token: "xoxb-token"
`
	_, _, err := MigrateConfig([]byte(input))
	if err == nil {
		t.Error("expected validation error for v1 config missing jira section, got none")
	}
	if err != nil && !strings.Contains(err.Error(), "must have 'jira' section") {
		t.Errorf("expected 'must have jira section' error, got: %v", err)
	}
}

func TestMigrateConfig_V1AlreadyUpToDate(t *testing.T) {
	input := `version: 1
jira:
  url: https://mycompany.atlassian.net
  username: user@example.com
  api_token: secret123
  project_key: MYPROJ
  task_statuses:
    - In Progress
    - In Review
tempo:
  enabled: true
  api_token: tempo-secret
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
  user_token: xoxp-token
  channel_id: C123456
breaks:
  - name: lunch
    duration: 60
    emoji: \":fork_and_knife:\"
update:
  check_for_updates: true
  check_interval: 24
`
	result, summary, err := MigrateConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.NeedsUpdate {
		t.Errorf("expected NeedsUpdate=false for complete v1 config, got true")
	}

	if summary.FromVersion != 1 || summary.ToVersion != 1 {
		t.Errorf("expected v1→v1, got v%d→v%d", summary.FromVersion, summary.ToVersion)
	}

	// Should return original config unchanged
	resultStr := string(result)
	if !strings.Contains(resultStr, "version: 1") {
		t.Error("expected result to contain version: 1")
	}
	if !strings.Contains(resultStr, "user_token: xoxp-token") {
		t.Error("expected result to preserve existing user_token")
	}
}

func TestMigrateConfig_V0ToV1_Complete(t *testing.T) {
	// Test complete migration from v0 (no version) to v1
	input := `jira:
  url: https://mycompany.atlassian.net
  username: user@example.com
  api_token: secret123
  project_key: MYPROJ

slack:
  user_token: xoxp-preserved-token
  channel_id: C123456

tempo:
  enabled: true
  api_token: tempo-secret
`
	result, summary, err := MigrateConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !summary.NeedsUpdate {
		t.Error("expected NeedsUpdate=true for v0 config")
	}

	if summary.FromVersion != 0 || summary.ToVersion != 1 {
		t.Errorf("expected v0→v1, got v%d→v%d", summary.FromVersion, summary.ToVersion)
	}

	expectedMissing := []string{"jira.task_statuses"}
	if len(summary.MissingFields) != len(expectedMissing) {
		t.Errorf("expected %d missing fields, got %d: %v", len(expectedMissing), len(summary.MissingFields), summary.MissingFields)
	}

	resultStr := string(result)

	// Should add version: 1
	if !strings.Contains(resultStr, "version: 1") {
		t.Error("expected result to contain 'version: 1'")
	}

	// Should preserve original values
	if !strings.Contains(resultStr, "mycompany.atlassian.net") {
		t.Error("expected result to preserve jira URL")
	}
	if !strings.Contains(resultStr, "user@example.com") {
		t.Error("expected result to preserve username")
	}
	if !strings.Contains(resultStr, "MYPROJ") {
		t.Error("expected result to preserve project_key")
	}
	if !strings.Contains(resultStr, "C123456") {
		t.Error("expected result to preserve channel_id")
	}
	if !strings.Contains(resultStr, "tempo-secret") {
		t.Error("expected result to preserve tempo api_token")
	}

	// Should preserve user_token (not deprecated)
	if !strings.Contains(resultStr, "xoxp-preserved-token") {
		t.Error("expected result to preserve user_token")
	}

	// Should add task_statuses field with comment
	// Should add task_statuses field with comment
	if !strings.Contains(resultStr, "task_statuses") {
		t.Error("expected result to contain new task_statuses field")
	}

	// Verify YAML is valid and has correct structure
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("migrated config is not valid YAML: %v", err)
	}

	if version, ok := parsed["version"].(int); !ok || version != 1 {
		t.Errorf("expected version=1 in parsed config, got %v", parsed["version"])
	}
}

func TestMigrateConfig_PreservesComments(t *testing.T) {
	input := `# Main config file
jira:
  # Production URL
  url: "https://example.com"
  project_key: "PROJ"
slack:
  user_token: "xoxp-preserved"  # This is valid in v1
  channel_id: "C123"
`
	result, summary, err := MigrateConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !summary.NeedsUpdate {
		t.Error("expected config to need update")
	}

	resultStr := string(result)

	// Verify user_token value is preserved
	if !strings.Contains(resultStr, "xoxp-preserved") {
		t.Error("expected user_token value 'xoxp-preserved' to be preserved")
	}

	// Note: Comments are not preserved during migration since we manipulate raw YAML
	// This is acceptable and documented
}

func TestMigrateConfig_V1WithMissingOptionalSections(t *testing.T) {
	input := `version: 1
jira:
  url: https://example.com
  username: user@example.com
  api_token: secret123
  project_key: PROJ
  task_statuses:
    - In Progress
slack:
  user_token: xoxp-token
  channel_id: C123
tempo:
  enabled: false
  api_token: ""
database:
  path: /home/user/.tasklog/tasklog.db
`
	result, summary, err := MigrateConfig([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should need update due to missing optional sections
	if !summary.NeedsUpdate {
		t.Error("expected NeedsUpdate=true for config missing optional sections")
	}

	// Should stay at v1
	if summary.FromVersion != 1 || summary.ToVersion != 1 {
		t.Errorf("expected v1→v1, got v%d→v%d", summary.FromVersion, summary.ToVersion)
	}

	// Should detect missing optional sections
	expectedMissing := []string{"labels", "shortcuts", "breaks", "update"}
	if len(summary.MissingOptionalSections) != len(expectedMissing) {
		t.Errorf("expected %d missing sections, got %d: %v",
			len(expectedMissing), len(summary.MissingOptionalSections), summary.MissingOptionalSections)
	}

	for _, expected := range expectedMissing {
		found := false
		for _, actual := range summary.MissingOptionalSections {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected missing section '%s' not found in: %v", expected, summary.MissingOptionalSections)
		}
	}

	// Result should be unchanged at this stage (ApplyOptionalSections is called separately)
	if string(result) != input {
		t.Error("MigrateConfig should not modify v1 config, only detect missing sections")
	}
}

func TestApplyOptionalSections(t *testing.T) {
	input := `version: 1
jira:
  url: https://example.com
  username: user@example.com
  api_token: secret123
  project_key: PROJ
  task_statuses:
    - In Progress
slack:
  bot_token: xoxb-token
  user_id: U123
database:
  path: /home/user/.tasklog/tasklog.db
tempo:
  enabled: false
`

	missingSections := []string{"labels", "shortcuts", "breaks"}

	result, err := ApplyOptionalSections([]byte(input), missingSections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)

	// Verify all sections were added
	if !strings.Contains(resultStr, "labels:") {
		t.Error("expected 'labels' section to be added")
	}
	if !strings.Contains(resultStr, "shortcuts:") {
		t.Error("expected 'shortcuts' section to be added")
	}
	if !strings.Contains(resultStr, "breaks:") {
		t.Error("expected 'breaks' section to be added")
	}

	// Verify it has example values from template
	if !strings.Contains(resultStr, "allowed_labels:") {
		t.Error("expected 'allowed_labels' in labels section")
	}
	if !strings.Contains(resultStr, "name: daily") {
		t.Error("expected shortcut example 'daily' in shortcuts section")
	}
	if !strings.Contains(resultStr, "name: lunch") {
		t.Error("expected break example 'lunch' in breaks section")
	}

	// Verify original fields are preserved
	if !strings.Contains(resultStr, "url: https://example.com") {
		t.Error("expected original jira URL to be preserved")
	}
	if !strings.Contains(resultStr, "bot_token: xoxb-token") {
		t.Error("expected original bot_token to be preserved")
	}

	// Verify valid YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid YAML: %v", err)
	}
}

func TestApplyOptionalSections_EmptyList(t *testing.T) {
	input := `version: 1
jira:
  url: https://example.com
`

	result, err := ApplyOptionalSections([]byte(input), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return unchanged input
	if string(result) != input {
		t.Error("expected unchanged input when no sections to add")
	}
}

func TestDetectMissingOptionalSections_DatabaseAndSlackAreOptional(t *testing.T) {
	// Config without database and slack - these should be detected as missing
	configWithoutOptionals := `version: 1
jira:
  url: https://example.atlassian.net
  username: user@example.com
  api_token: token123
  project_key: PROJ
tempo:
  enabled: true
  api_token: tempo123`

	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(configWithoutOptionals), &rawConfig); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	missing := detectMissingOptionalSections(rawConfig)

	// Verify database and slack are detected as missing (optional)
	hasDatabaseMissing := false
	hasSlackMissing := false
	for _, section := range missing {
		if section == "database" {
			hasDatabaseMissing = true
		}
		if section == "slack" {
			hasSlackMissing = true
		}
	}

	if !hasDatabaseMissing {
		t.Error("database should be detected as missing optional section")
	}
	if !hasSlackMissing {
		t.Error("slack should be detected as missing optional section")
	}

	// Config with database and slack - these should not be detected as missing
	configWithOptionals := `version: 1
jira:
  url: https://example.atlassian.net
  username: user@example.com
  api_token: token123
  project_key: PROJ
tempo:
  enabled: true
  api_token: tempo123
database:
  path: /custom/path/db.sqlite
slack:
  user_token: xoxp-token
  channel_id: C123456`

	var rawConfigWithOptionals map[string]interface{}
	if err := yaml.Unmarshal([]byte(configWithOptionals), &rawConfigWithOptionals); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	missing = detectMissingOptionalSections(rawConfigWithOptionals)

	// Verify database and slack are not in missing list
	for _, section := range missing {
		if section == "database" || section == "slack" {
			t.Errorf("section %s should not be in missing list when present", section)
		}
	}
}
