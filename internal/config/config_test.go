package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Jira: JiraConfig{
					URL:        "https://example.atlassian.net",
					Username:   "user@example.com",
					APIToken:   "token123",
					ProjectKey: "PROJ",
				},
				Tempo: TempoConfig{
					APIToken: "tempo-token",
				},
			},
			wantError: false,
		},
		{
			name: "missing jira url",
			config: Config{
				Jira: JiraConfig{
					Username:   "user@example.com",
					APIToken:   "token123",
					ProjectKey: "PROJ",
				},
				Tempo: TempoConfig{
					APIToken: "tempo-token",
				},
			},
			wantError: true,
			errorMsg:  "jira.url is required",
		},
		{
			name: "missing jira username",
			config: Config{
				Jira: JiraConfig{
					URL:        "https://example.atlassian.net",
					APIToken:   "token123",
					ProjectKey: "PROJ",
				},
				Tempo: TempoConfig{
					APIToken: "tempo-token",
				},
			},
			wantError: true,
			errorMsg:  "jira.username is required",
		},
		{
			name: "missing jira api token",
			config: Config{
				Jira: JiraConfig{
					URL:        "https://example.atlassian.net",
					Username:   "user@example.com",
					ProjectKey: "PROJ",
				},
				Tempo: TempoConfig{
					APIToken: "tempo-token",
				},
			},
			wantError: true,
			errorMsg:  "jira.api_token is required",
		},
		{
			name: "missing jira project key",
			config: Config{
				Jira: JiraConfig{
					URL:      "https://example.atlassian.net",
					Username: "user@example.com",
					APIToken: "token123",
				},
				Tempo: TempoConfig{
					APIToken: "tempo-token",
				},
			},
			wantError: true,
			errorMsg:  "jira.project_key is required",
		},
		{
			name: "missing tempo api token",
			config: Config{
				Jira: JiraConfig{
					URL:        "https://example.atlassian.net",
					Username:   "user@example.com",
					APIToken:   "token123",
					ProjectKey: "PROJ",
				},
				Tempo: TempoConfig{
					Enabled: true,
				},
			},
			wantError: true,
			errorMsg:  "tempo.api_token is required when tempo.enabled is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetShortcut(t *testing.T) {
	config := Config{
		Shortcuts: []ShortcutEntry{
			{Name: "daily", Task: "PROJ-123", Time: "30m", Label: "meeting"},
			{Name: "standup", Task: "PROJ-456", Time: "15m", Label: "meeting"},
		},
	}

	tests := []struct {
		name      string
		shortcut  string
		wantFound bool
		wantTask  string
	}{
		{"existing shortcut", "daily", true, "PROJ-123"},
		{"another shortcut", "standup", true, "PROJ-456"},
		{"non-existent shortcut", "nonexistent", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortcut, found := config.GetShortcut(tt.shortcut)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && shortcut.Task != tt.wantTask {
				t.Errorf("expected task %s, got %s", tt.wantTask, shortcut.Task)
			}
		})
	}
}

func TestIsLabelAllowed(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		label   string
		allowed bool
	}{
		{
			name: "label in allowed list",
			config: Config{
				Labels: LabelsConfig{
					AllowedLabels: []string{"development", "testing", "meeting"},
				},
			},
			label:   "development",
			allowed: true,
		},
		{
			name: "label not in allowed list",
			config: Config{
				Labels: LabelsConfig{
					AllowedLabels: []string{"development", "testing"},
				},
			},
			label:   "meeting",
			allowed: false,
		},
		{
			name: "empty allowed list allows all",
			config: Config{
				Labels: LabelsConfig{
					AllowedLabels: []string{},
				},
			},
			label:   "anything",
			allowed: true,
		},
		{
			name:    "no labels config allows all",
			config:  Config{},
			label:   "anything",
			allowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsLabelAllowed(tt.label)
			if result != tt.allowed {
				t.Errorf("expected %v, got %v", tt.allowed, result)
			}
		})
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// This test creates a temporary directory to avoid affecting user's home
	tmpDir := t.TempDir()

	// We can't easily test the actual function since it uses os.UserHomeDir()
	// But we can test the directory creation logic
	testDir := filepath.Join(tmpDir, ".tasklog")

	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Check if directory was created
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	// Set environment variable to non-existent file
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.yaml")
	os.Setenv("TASKLOG_CONFIG", nonExistentPath)
	defer os.Unsetenv("TASKLOG_CONFIG")

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
jira:
  url: https://example.com
  invalid yaml here: [
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0600)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("TASKLOG_CONFIG", configPath)
	defer os.Unsetenv("TASKLOG_CONFIG")

	_, err = Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	// Create a temporary config file with valid configuration
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := `
jira:
  url: "https://example.atlassian.net"
  username: "user@example.com"
  api_token: "token123"
  project_key: "PROJ"
  task_statuses:
    - "In Progress"
    - "In Review"

tempo:
  api_token: "tempo-token"

labels:
  allowed_labels:
    - "development"
    - "testing"

shortcuts:
  - name: "daily"
    task: "PROJ-123"
    time: "30m"
    label: "meeting"
`
	err := os.WriteFile(configPath, []byte(validConfig), 0600)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("TASKLOG_CONFIG", configPath)
	defer os.Unsetenv("TASKLOG_CONFIG")

	config, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading valid config: %v", err)
	}

	if config.Jira.URL != "https://example.atlassian.net" {
		t.Errorf("expected jira url to be loaded correctly")
	}

	if config.Jira.ProjectKey != "PROJ" {
		t.Errorf("expected project key PROJ, got %s", config.Jira.ProjectKey)
	}

	if len(config.Jira.TaskStatuses) != 2 {
		t.Errorf("expected 2 task statuses, got %d", len(config.Jira.TaskStatuses))
	}

	if len(config.Jira.TaskStatuses) > 0 && config.Jira.TaskStatuses[0] != "In Progress" {
		t.Errorf("expected first task status to be 'In Progress', got %s", config.Jira.TaskStatuses[0])
	}

	if len(config.Jira.TaskStatuses) > 1 && config.Jira.TaskStatuses[1] != "In Review" {
		t.Errorf("expected second task status to be 'In Review', got %s", config.Jira.TaskStatuses[1])
	}

	if len(config.Labels.AllowedLabels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(config.Labels.AllowedLabels))
	}

	if len(config.Shortcuts) != 1 {
		t.Errorf("expected 1 shortcut, got %d", len(config.Shortcuts))
	}
}

func TestLoadConfig_WithoutTaskStatuses(t *testing.T) {
	// Test that config loads successfully when task_statuses is not specified
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configWithoutStatuses := `
jira:
  url: "https://example.atlassian.net"
  username: "user@example.com"
  api_token: "token123"
  project_key: "PROJ"

tempo:
  api_token: "tempo-token"
`
	err := os.WriteFile(configPath, []byte(configWithoutStatuses), 0600)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("TASKLOG_CONFIG", configPath)
	defer os.Unsetenv("TASKLOG_CONFIG")

	config, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config without task_statuses: %v", err)
	}

	if config.Jira.URL != "https://example.atlassian.net" {
		t.Errorf("expected jira url to be loaded correctly")
	}

	// TaskStatuses should be empty/nil when not specified
	if len(config.Jira.TaskStatuses) != 0 {
		t.Errorf("expected 0 task statuses when not specified, got %d", len(config.Jira.TaskStatuses))
	}
}

func TestConfig_GetBreak(t *testing.T) {
	config := &Config{
		Breaks: []BreakEntry{
			{Name: "lunch", Duration: 60, Emoji: ":fork_and_knife:"},
			{Name: "prayer", Duration: 15, Emoji: ":pray:"},
			{Name: "coffee", Duration: 10, Emoji: ":coffee:"},
		},
	}

	tests := []struct {
		name         string
		breakName    string
		wantFound    bool
		wantDuration int
	}{
		{"existing break", "lunch", true, 60},
		{"another break", "prayer", true, 15},
		{"non-existent break", "vacation", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breakEntry, found := config.GetBreak(tt.breakName)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && breakEntry.Duration != tt.wantDuration {
				t.Errorf("expected duration %d, got %d", tt.wantDuration, breakEntry.Duration)
			}
		})
	}
}

func TestEnsureConfigDir_RespectsEnvVar(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	customConfigPath := filepath.Join(tmpDir, "custom", "subdir", "config.yaml")

	// Set TASKLOG_CONFIG environment variable
	os.Setenv("TASKLOG_CONFIG", customConfigPath)
	defer os.Unsetenv("TASKLOG_CONFIG")

	// Call EnsureConfigDir
	err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir failed: %v", err)
	}

	// Verify the directory was created
	expectedDir := filepath.Join(tmpDir, "custom", "subdir")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected directory %s to be created, but it does not exist", expectedDir)
	}
}
