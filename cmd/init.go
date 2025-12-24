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
		fmt.Printf("Config file already exists at: %s\n", configPath)
		fmt.Println("\nHelpful commands:")
		fmt.Println("  tasklog config example  - View the complete example config with all options")
		fmt.Println("  tasklog config show     - Display your current configuration")
		fmt.Println("  tasklog config compare  - Compare your config with the example to find missing fields")
		fmt.Println("\nTo reinitialize, delete the existing file and run this command again.")
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
	fmt.Println("✓ Configuration initialized successfully!")
	fmt.Printf("\nConfig file created at: %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the config file with your Jira and Tempo credentials")
	fmt.Println("2. Set the Jira project_key for your project (required)")
	fmt.Println("3. Get your Jira API token: https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println("4. Get your Tempo API token from Tempo > Settings > API Integration")
	fmt.Println("5. (Optional) Configure labels and shortcuts")
	fmt.Printf("6. Run: tasklog log\n")
}

// printError prints an error message and returns nil (for cobra command compatibility)
func printError(message string, err error) error {
	fmt.Printf("%s: %v\n", message, err)
	return nil
}
