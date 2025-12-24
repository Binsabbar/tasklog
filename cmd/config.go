package cmd

import (
	"fmt"
	"os"

	"tasklog/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Commands for managing and viewing tasklog configuration.`,
}

var configExampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Display example configuration",
	Long: `Displays the complete example configuration with all available options.

Use this to:
- See what configuration options are available
- Compare against your existing config to find missing fields
- Copy sections to add to your own config file`,
	RunE: runConfigExample,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long: `Displays your current configuration file.

This shows the raw YAML content of your config file at ~/.tasklog/config.yaml
(or the path specified by TASKLOG_CONFIG environment variable).`,
	RunE: runConfigShow,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configExampleCmd)
	configCmd.AddCommand(configShowCmd)
}

func runConfigExample(cmd *cobra.Command, args []string) error {
	// Generate example config
	exampleData, err := config.GenerateExampleConfig()
	if err != nil {
		return fmt.Errorf("failed to generate example config: %w", err)
	}

	// Print the example config
	fmt.Println("# Example tasklog configuration with all available options:")
	fmt.Println("# Copy relevant sections to your config file at ~/.tasklog/config.yaml")
	fmt.Println()
	fmt.Print(string(exampleData))

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Get config path
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s\nRun 'tasklog init' to create one", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Print config path and content
	fmt.Printf("# Configuration file: %s\n\n", configPath)
	fmt.Print(string(data))

	return nil
}