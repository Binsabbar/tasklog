package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasklog/internal/config"

	"gopkg.in/yaml.v3"
)

// TestCreateNewConfig tests creating a new config file
func TestCreateNewConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := createNewConfig(configPath)
	if err != nil {
		t.Fatalf("createNewConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Verify it contains expected content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}

	content := string(data)

	// Check for key sections
	expectedSections := []string{
		"version: 1",
		"jira:",
		"shortcuts:",
		"slack:",
		"breaks:",
		"tempo:",
		"labels:",
		"database:",
		"update:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(content, section) {
			t.Errorf("expected config to contain %q", section)
		}
	}
}

// TestGenerateExampleConfig tests the example config generation
func TestGenerateExampleConfig(t *testing.T) {
	data, err := config.GenerateExampleConfig()
	if err != nil {
		t.Fatalf("GenerateExampleConfig failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("GenerateExampleConfig returned empty data")
	}

	content := string(data)

	// Verify it's valid YAML with expected structure
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("generated config is not valid YAML: %v", err)
	}

	// Verify version
	if cfg.Version != config.CurrentConfigVersion {
		t.Errorf("expected version %d, got %d", config.CurrentConfigVersion, cfg.Version)
	}

	// Verify nested structure
	if len(cfg.Jira.Shortcuts) == 0 {
		t.Error("expected jira.shortcuts to be populated in example")
	}

	if len(cfg.Slack.Breaks) == 0 {
		t.Error("expected slack.breaks to be populated in example")
	}

	// Verify it contains helpful comments
	contentLower := strings.ToLower(content)
	if !strings.Contains(contentLower, "configuration") || !strings.Contains(contentLower, "optional") {
		t.Error("expected example config to contain helpful comments")
	}
}
