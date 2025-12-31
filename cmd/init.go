package cmd

import (
	"fmt"
	"os"

	"tasklog/internal/config"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize tasklog configuration",
	Long: `Creates the configuration directory and an example config file at ~/.tasklog/config.yaml

If a config file already exists, use 'tasklog config example' to view the template
and update your config manually.

Helpful commands:
  tasklog config example  - View the complete example config with all options
  tasklog config show     - Display your current configuration
  tasklog config compare  - Compare your config with the example to find missing fields` + configHelp,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get config path (respects TASKLOG_CONFIG environment variable)
	configPath, err := config.GetConfigPath()
	if err != nil {
		return printError("failed to get config path", err)
	}

	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return printError("failed to create config directory", err)
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		Out.Printf("Config file already exists at: %s\n", configPath)
		Out.Println("\nHelpful commands:")
		Out.Println("  tasklog config example  - View the complete example config with all options")
		Out.Println("  tasklog config show     - Display your current configuration")
		Out.Println("  tasklog config compare  - Compare your config with the example to find missing fields")
		Out.Println("\nTo reinitialize, delete the existing file and run this command again.")
		return nil
	}

	return createNewConfig(configPath)
}

// createNewConfig generates and writes a new config file
func createNewConfig(configPath string) error {
	// Generate example config from the Config struct
	exampleData, err := config.GenerateExampleConfig()
	if err != nil {
		return printError("failed to generate example config", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, exampleData, 0600); err != nil {
		return printError("failed to create config file", err)
	}

	printSuccessMessage(configPath)
	return nil
}

// printSuccessMessage displays the success message after config creation
func printSuccessMessage(configPath string) {
	Out.Success("Configuration initialized successfully!")
	Out.Printf("\nConfig file created at: %s\n", configPath)
	Out.Println("\nNext steps:")
	Out.Println("1. Edit the config file with your Jira and Tempo credentials")
	Out.Println("2. Set the Jira project_key for your project (required)")
	Out.Println("3. Get your Jira API token: https://id.atlassian.com/manage-profile/security/api-tokens")
	Out.Println("4. Get your Tempo API token from Tempo > Settings > API Integration")
	Out.Println("5. (Optional) Configure labels and shortcuts")
	Out.Printf("6. Run: tasklog log\n")
}

// printError prints an error message and returns nil (for cobra command compatibility)
func printError(message string, err error) error {
	Out.Error(fmt.Errorf("%s: %w", message, err))
	return nil
}
