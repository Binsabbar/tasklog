package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Jira      JiraConfig      `yaml:"jira"`
	Tempo     TempoConfig     `yaml:"tempo"`
	Labels    LabelsConfig    `yaml:"labels"`
	Shortcuts []ShortcutEntry `yaml:"shortcuts"`
	Database  DatabaseConfig  `yaml:"database"`
	Slack     SlackConfig     `yaml:"slack"`
	Breaks    []BreakEntry    `yaml:"breaks"`
}

// JiraConfig contains Jira API configuration (all fields required)
type JiraConfig struct {
	URL          string   `yaml:"url"`           // Jira instance URL (required)
	Username     string   `yaml:"username"`      // Jira username/email (required)
	APIToken     string   `yaml:"api_token"`     // Jira API token (required)
	ProjectKey   string   `yaml:"project_key"`   // Project key to filter tasks (required)
	TaskStatuses []string `yaml:"task_statuses"` // Task statuses to include (optional, defaults to ["In Progress"])
}

// TempoConfig contains Tempo API configuration (optional)
type TempoConfig struct {
	APIToken string `yaml:"api_token"` // Tempo API token (optional - only if logging separately to Tempo)
	Enabled  bool   `yaml:"enabled"`   // Whether to log to Tempo separately (optional, default: false)
}

// LabelsConfig contains label filtering configuration (optional)
type LabelsConfig struct {
	AllowedLabels []string `yaml:"allowed_labels"` // List of allowed labels from Jira (optional)
}

// ShortcutEntry represents a predefined shortcut for quick time logging (optional)
type ShortcutEntry struct {
	Name  string `yaml:"name"`  // Shortcut name (e.g., "daily")
	Task  string `yaml:"task"`  // Jira task key (e.g., "PROJ-123")
	Time  string `yaml:"time"`  // Optional: predefined time (e.g., "30m")
	Label string `yaml:"label"` // Work log label
}

// DatabaseConfig contains SQLite database configuration (optional)
type DatabaseConfig struct {
	Path string `yaml:"path"` // Path to SQLite database file (optional, defaults to ~/.tasklog/tasklog.db)
}

// SlackConfig contains Slack integration configuration (optional)
type SlackConfig struct {
	UserToken string `yaml:"user_token"` // Slack user OAuth token (optional)
	ChannelID string `yaml:"channel_id"` // Channel ID for break messages (optional)
}

// BreakEntry represents a predefined break type (optional)
type BreakEntry struct {
	Name     string `yaml:"name"`     // Break name (e.g., "lunch", "prayer")
	Duration int    `yaml:"duration"` // Duration in minutes
	Emoji    string `yaml:"emoji"`    // Emoji for Slack status (optional)
}

// Load loads configuration from the config file
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	log.Debug().Str("path", configPath).Msg("Loading configuration")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s. Please create one using the example", configPath)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Database.Path == "" {
		config.Database.Path = filepath.Join(getConfigDir(), "tasklog.db")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	log.Debug().Msg("Configuration loaded successfully")
	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Jira.URL == "" {
		return fmt.Errorf("jira.url is required")
	}
	if c.Jira.Username == "" {
		return fmt.Errorf("jira.username is required")
	}
	if c.Jira.APIToken == "" {
		return fmt.Errorf("jira.api_token is required")
	}
	if c.Jira.ProjectKey == "" {
		return fmt.Errorf("jira.project_key is required")
	}
	// Tempo is optional - only validate if enabled
	if c.Tempo.Enabled && c.Tempo.APIToken == "" {
		return fmt.Errorf("tempo.api_token is required when tempo.enabled is true")
	}
	return nil
}

// GetShortcut returns a shortcut by name
func (c *Config) GetShortcut(name string) (*ShortcutEntry, bool) {
	for _, shortcut := range c.Shortcuts {
		if shortcut.Name == name {
			return &shortcut, true
		}
	}
	return nil, false
}

// IsLabelAllowed checks if a label is in the allowed list
// If no allowed labels are configured, all labels are allowed
func (c *Config) IsLabelAllowed(label string) bool {
	if len(c.Labels.AllowedLabels) == 0 {
		return true
	}
	for _, allowed := range c.Labels.AllowedLabels {
		if allowed == label {
			return true
		}
	}
	return false
}

// GetBreak returns a break by name
func (c *Config) GetBreak(name string) (*BreakEntry, bool) {
	for _, breakEntry := range c.Breaks {
		if breakEntry.Name == name {
			return &breakEntry, true
		}
	}
	return nil, false
}

// getConfigDir returns the configuration directory path
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get user home directory")
	}
	return filepath.Join(homeDir, ".tasklog")
}

// getConfigPath returns the full path to the config file
func getConfigPath() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("TASKLOG_CONFIG"); envPath != "" {
		return envPath, nil
	}

	// Otherwise use default path
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, "config.yaml")

	return configPath, nil
}

// EnsureConfigDir ensures the config directory exists
func EnsureConfigDir() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return nil
}
