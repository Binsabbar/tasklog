package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// CurrentConfigVersion is the latest config schema version
// v1: Initial versioned schema with nested structure (shortcuts under jira, breaks under slack)
const CurrentConfigVersion = 1

// Config represents the application configuration
type Config struct {
	Version  int            `yaml:"version,omitempty"` // Schema version for migrations
	Jira     JiraConfig     `yaml:"jira"`
	Tempo    TempoConfig    `yaml:"tempo"`
	Labels   LabelsConfig   `yaml:"labels"`
	Database DatabaseConfig `yaml:"database"`
	Slack    SlackConfig    `yaml:"slack"`
	Update   UpdateConfig   `yaml:"update"` // Update checking configuration (optional)
}

// JiraConfig contains Jira API configuration (all fields required)
type JiraConfig struct {
	URL          string          `yaml:"url" validate:"required,url"`        // Jira instance URL (required)
	Username     string          `yaml:"username" validate:"required,email"` // Jira username/email (required)
	APIToken     string          `yaml:"api_token" validate:"required"`      // Jira API token (required)
	ProjectKey   string          `yaml:"project_key" validate:"required"`    // Project key to filter tasks (required)
	TaskStatuses []string        `yaml:"task_statuses"`                      // Task statuses to include (optional, defaults to ["In Progress"])
	Shortcuts    []ShortcutEntry `yaml:"shortcuts"`                          // Predefined shortcuts for quick time logging (optional)
}

// TempoConfig contains Tempo API configuration (optional)
type TempoConfig struct {
	APIToken string `yaml:"api_token" validate:"required_if=Enabled true"` // Tempo API token (optional - only if logging separately to Tempo)
	Enabled  bool   `yaml:"enabled"`                                       // Whether to log to Tempo separately (optional, default: false)
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
	UserToken string       `yaml:"user_token"` // Slack user OAuth token (optional)
	ChannelID string       `yaml:"channel_id"` // Channel ID for break messages (optional)
	Breaks    []BreakEntry `yaml:"breaks"`     // Predefined break types (optional)
}

// BreakEntry represents a predefined break type (optional)
type BreakEntry struct {
	Name     string `yaml:"name"`     // Break name (e.g., "lunch", "prayer")
	Duration int    `yaml:"duration"` // Duration in minutes
	Emoji    string `yaml:"emoji"`    // Emoji for Slack status (optional)
}

// UpdateConfig contains update checking configuration (optional)
type UpdateConfig struct {
	Disabled      bool   `yaml:"disabled"`       // Whether to disable update checking (default: false, meaning checks are enabled)
	CheckInterval string `yaml:"check_interval"` // Check interval as duration string like "24h", "1d" (default: "24h")
	Channel       string `yaml:"channel"`        // Release channel: "", "stable", "alpha", "beta", "rc" (default: auto-detect from current version)
}

// Load loads configuration from the config file
func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	log.Debug().Str("path", configPath).Msg("Loading configuration")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s. Please create one using `tasklog init` command", configPath)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Database.Path == "" {
		config.Database.Path = filepath.Join(getDefaultConfigDir(), "tasklog.db")
	}

	// Set update config defaults
	if config.Update.CheckInterval == "" {
		config.Update.CheckInterval = "24h" // Default: check once per day
	}
	// Disabled defaults to false (meaning update checks are enabled by default)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	log.Debug().Msg("Configuration loaded successfully")
	return &config, nil
}

// Validate validates the configuration using struct tags
func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		// Format validation errors to be more user-friendly
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, fieldErr := range validationErrors {
				// Convert field namespace to yaml-style path (e.g., Config.Jira.URL -> jira.url)
				field := convertFieldNameToYAMLPath(fieldErr.Namespace())

				switch fieldErr.Tag() {
				case "required":
					return fmt.Errorf("%s is required", field)
				case "url":
					return fmt.Errorf("%s must be a valid URL", field)
				case "email":
					return fmt.Errorf("%s must be a valid email address", field)
				case "required_if":
					// Extract the field name from the parameter (e.g., "Enabled true" -> "enabled is true")
					return fmt.Errorf("%s is required when %s.enabled is true", field, "tempo")
				default:
					return fmt.Errorf("%s failed validation: %s", field, fieldErr.Tag())
				}
			}
		}
		return err
	}
	return nil
}

// convertFieldNameToYAMLPath converts validator field path to yaml-style path
// Example: Config.Jira.URL -> jira.url, Config.Jira.APIToken -> jira.api_token
func convertFieldNameToYAMLPath(namespace string) string {
	// Remove "Config." prefix
	if len(namespace) > 7 && namespace[:7] == "Config." {
		namespace = namespace[7:]
	}

	// Convert camelCase/PascalCase to snake_case and lowercase
	result := ""
	for i, r := range namespace {
		if r == '.' {
			result += string(r)
		} else if r >= 'A' && r <= 'Z' {
			// Check if this is the start of a new segment (after a dot)
			if i > 0 && namespace[i-1] == '.' {
				// First letter after dot - just lowercase
				result += string(r + 32)
			} else if i > 0 && namespace[i-1] >= 'A' && namespace[i-1] <= 'Z' && i+1 < len(namespace) && namespace[i+1] >= 'a' && namespace[i+1] <= 'z' {
				// Handle acronyms like "API" -> "api_" (when followed by lowercase)
				result += "_" + string(r+32)
			} else if i > 0 && namespace[i-1] >= 'a' && namespace[i-1] <= 'z' {
				// Transition from lowercase to uppercase
				result += "_" + string(r+32)
			} else {
				// Part of acronym or first char - just lowercase
				result += string(r + 32)
			}
		} else {
			result += string(r)
		}
	}
	return result
}

// GetShortcut returns a shortcut by name
func (c *Config) GetShortcut(name string) (*ShortcutEntry, bool) {
	for _, shortcut := range c.Jira.Shortcuts {
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
	for _, breakEntry := range c.Slack.Breaks {
		if breakEntry.Name == name {
			return &breakEntry, true
		}
	}
	return nil, false
}

// getDefaultConfigDir returns the configuration directory path
func getDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get user home directory")
	}
	return filepath.Join(homeDir, ".tasklog")
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("TASKLOG_CONFIG"); envPath != "" {
		return envPath, nil
	}

	// Otherwise use default path
	configDir := getDefaultConfigDir()
	configPath := filepath.Join(configDir, "config.yaml")

	return configPath, nil
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(configPath), nil
}

// EnsureConfigDir ensures the config directory exists
// If TASKLOG_CONFIG is set, it ensures the directory for that file exists
func EnsureConfigDir() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Extract directory from the config path
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return nil
}
