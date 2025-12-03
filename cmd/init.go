package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasklog/internal/config"

	"github.com/spf13/cobra"
)

var (
	updateConfig bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize tasklog configuration",
	Long:  `Creates the configuration directory and an example config file at ~/.tasklog/config.yaml`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&updateConfig, "update", false, "Update existing config file with missing fields")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get config path (respects TASKLOG_CONFIG environment variable)
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		if updateConfig {
			return updateExistingConfig(configPath)
		}
		fmt.Printf("Config file already exists at: %s\n", configPath)
		fmt.Println("To update the config with new fields, run: tasklog init --update")
		fmt.Println("To reinitialize, delete the existing file and run this command again.")
		return nil
	}

	// If --update flag is used but no config exists, create a new one
	if updateConfig {
		fmt.Println("No existing config file found. Creating a new one...")
	}

	// Read example config from current directory
	examplePath := "config.example.yaml"
	exampleData, err := os.ReadFile(examplePath)
	if err != nil {
		// If example doesn't exist in current directory, create a basic one
		exampleData = []byte(`# Tasklog Configuration
# Fill in your credentials below

# Required: Jira configuration
jira:
  url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_token: "your-jira-api-token"
  project_key: "PROJ"  # Project key to filter tasks

# Required: Tempo configuration
tempo:
  api_token: "your-tempo-api-token"

# Optional: Filter labels that can be used for time logging
labels:
  allowed_labels:
    - "development"
    - "code-review"
    - "meeting"
    - "testing"
    - "documentation"
    - "bug-fix"

# Optional: Define shortcuts for quick time logging
shortcuts:
  - name: "daily"
    task: "PROJ-123"
    time: "30m"
    label: "meeting"
  
  - name: "standup"
    task: "PROJ-123"
    time: "15m"
    label: "meeting"

# Optional: Database path (defaults to ~/.tasklog/tasklog.db)
database:
  path: ""
`)
	}

	// Write config file
	if err := os.WriteFile(configPath, exampleData, 0600); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	fmt.Println("✓ Configuration initialized successfully!")
	fmt.Printf("\nConfig file created at: %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the config file with your Jira and Tempo credentials")
	fmt.Println("2. Set the Jira project_key for your project (required)")
	fmt.Println("3. Get your Jira API token: https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println("4. Get your Tempo API token from Tempo > Settings > API Integration")
	fmt.Println("5. (Optional) Configure labels and shortcuts")
	fmt.Printf("6. Run: tasklog log\n")

	return nil
}

// updateExistingConfig updates an existing config file with new fields
func updateExistingConfig(configPath string) error {
	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Analyze what needs to be updated
	hasDeprecatedFields := false
	missingFields := []string{}

	// Check for deprecated fields
	if strings.Contains(content, "user_token:") {
		hasDeprecatedFields = true
	}

	// Check for missing new fields (task_statuses in jira section)
	if !strings.Contains(content, "task_statuses:") {
		missingFields = append(missingFields, "jira.task_statuses")
	}

	// If nothing needs updating, inform the user
	if !hasDeprecatedFields && len(missingFields) == 0 {
		fmt.Println("✓ Config file is already up to date!")
		return nil
	}

	// Show what will be changed
	fmt.Println("Config migration required:")
	if hasDeprecatedFields {
		fmt.Println("\n⚠️  Deprecated fields to be removed:")
		fmt.Println("  - slack.user_token (replaced by bot_token and user_id)")
	}
	if len(missingFields) > 0 {
		fmt.Println("\n✨ New optional fields to be added (commented out):")
		for _, field := range missingFields {
			fmt.Printf("  - %s\n", field)
		}
	}

	// Ask for confirmation
	fmt.Printf("\nA backup will be created at: %s.backup\n", configPath)
	if !confirm("Do you want to proceed with the update?") {
		fmt.Println("Update cancelled.")
		return nil
	}

	// Create backup
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("✓ Backup created at: %s\n", backupPath)

	// Update the config
	updatedLines := removeDeprecatedFields(lines)
	updatedLines = addNewFields(updatedLines)

	// Write updated config
	updatedContent := strings.Join(updatedLines, "\n")
	if err := os.WriteFile(configPath, []byte(updatedContent), 0600); err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	fmt.Println("✓ Config file updated successfully!")
	fmt.Println("\nChanges made:")
	if hasDeprecatedFields {
		fmt.Println("  - Removed deprecated fields")
	}
	if len(missingFields) > 0 {
		fmt.Println("  - Added new optional fields (commented out)")
		fmt.Println("\nReview the config file and uncomment/configure new fields as needed.")
	}

	return nil
}

// removeDeprecatedFields removes deprecated configuration fields
func removeDeprecatedFields(lines []string) []string {
	result := []string{}
	inSlackSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if we're in the slack section
		if trimmed == "slack:" {
			inSlackSection = true
			result = append(result, line)
			continue
		}

		// Exit slack section when we hit another top-level key
		if inSlackSection && len(trimmed) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(trimmed, "#") {
			inSlackSection = false
		}

		// Remove user_token from slack section
		if inSlackSection && strings.Contains(trimmed, "user_token:") {
			continue // Skip this line
		}

		result = append(result, line)
	}

	return result
}

// addNewFields adds new configuration fields as comments
func addNewFields(lines []string) []string {
	result := []string{}
	jiraSectionProcessed := false
	slackSectionProcessed := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		result = append(result, line)

		// Add task_statuses to jira section
		if !jiraSectionProcessed && strings.HasPrefix(trimmed, "project_key:") {
			// Add task_statuses after project_key
			result = append(result, "  # Optional: Task statuses to include when fetching tasks (defaults to [\"In Progress\"])")
			result = append(result, "  # task_statuses:")
			result = append(result, "  #   - \"In Progress\"")
			result = append(result, "  #   - \"In Review\"")
			jiraSectionProcessed = true
		}

		// Add bot_token and user_id to slack section
		if !slackSectionProcessed && trimmed == "slack:" {
			// Check if there's content in slack section
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				// If slack section is empty or only has channel_id, add the new fields
				if nextLine == "" || strings.HasPrefix(nextLine, "#") || strings.HasPrefix(nextLine, "channel_id:") {
					result = append(result, "  # bot_token: \"xoxb-your-slack-bot-token\"  # Slack Bot OAuth Token")
					result = append(result, "  # user_id: \"U1234567890\"  # Your Slack User ID for status updates")
					slackSectionProcessed = true
				}
			}
		}
	}

	return result
}

// confirm prompts the user for yes/no confirmation
func confirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/n): ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
