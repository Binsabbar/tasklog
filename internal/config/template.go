package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// GenerateExampleConfig creates an example configuration with helpful comments
func GenerateExampleConfig() ([]byte, error) {
	// Create example config structure with placeholder values
	exampleConfig := Config{
		Version: CurrentConfigVersion,
		Jira: JiraConfig{
			URL:        "https://your-domain.atlassian.net",
			Username:   "your-email@example.com",
			APIToken:   "your-jira-api-token",
			ProjectKey: "PROJ",
			TaskStatuses: []string{
				"In Progress",
				"In Review",
			},
		},
		Tempo: TempoConfig{
			Enabled:  false,
			APIToken: "",
		},
		Labels: LabelsConfig{
			AllowedLabels: []string{
				"development",
				"code-review",
				"meeting",
				"testing",
				"documentation",
				"bug-fix",
			},
		},
		Shortcuts: []ShortcutEntry{
			{
				Name:  "daily",
				Task:  "PROJ-123",
				Time:  "30m",
				Label: "meeting",
			},
			{
				Name:  "standup",
				Task:  "PROJ-123",
				Time:  "15m",
				Label: "meeting",
			},
			{
				Name:  "code-review",
				Task:  "PROJ-456",
				Time:  "",
				Label: "code-review",
			},
		},
		Database: DatabaseConfig{
			Path: "",
		},
		Slack: SlackConfig{
			UserToken: "xoxp-your-slack-user-token",
			ChannelID: "C1234567890",
		},
		Breaks: []BreakEntry{
			{
				Name:     "lunch",
				Duration: 60,
				Emoji:    ":fork_and_knife:",
			},
			{
				Name:     "prayer",
				Duration: 15,
				Emoji:    ":pray:",
			},
			{
				Name:     "coffee",
				Duration: 10,
				Emoji:    ":coffee:",
			},
		},
		Update: UpdateConfig{
			Disabled:      false, // false = update checks enabled (default)
			CheckInterval: "24h",
			Channel:       "", // Auto-detect from current version (stable if on stable, pre-release channel if on pre-release)
		},
	}

	// Encode to YAML node for comment manipulation
	var node yaml.Node
	if err := node.Encode(exampleConfig); err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}

	// Add helpful comments
	addConfigComments(&node)

	// Marshal with comments preserved
	result, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	return result, nil
}

// addConfigComments adds helpful comments to the configuration structure
func addConfigComments(node *yaml.Node) {
	if node.Kind != yaml.MappingNode || len(node.Content) == 0 {
		return
	}

	// Add header comment to the root mapping
	node.HeadComment = "Tasklog Configuration\nGet your Jira API token: https://id.atlassian.com/manage-profile/security/api-tokens\nGet your Tempo API token: Tempo > Settings > API Integration"

	// Add comments to each section
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		switch keyNode.Value {
		case "jira":
			valueNode.HeadComment = "Jira configuration (required)"
		case "tempo":
			valueNode.HeadComment = "Tempo configuration (optional - only if logging separately to Tempo)"
		case "labels":
			valueNode.HeadComment = "Allowed labels for time logging (optional - if empty, all Jira labels available)"
		case "shortcuts":
			valueNode.HeadComment = "Shortcuts for quick time logging (optional)"
		case "database":
			valueNode.HeadComment = "Database configuration (optional)"
		case "slack":
			valueNode.HeadComment = "Slack integration for break notifications (optional)"
		case "breaks":
			valueNode.HeadComment = "Break types for quick registration (optional)"
		case "update":
			valueNode.HeadComment = "Update checking configuration (optional)"
		}
	}
}
