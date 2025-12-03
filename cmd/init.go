package cmd

import (
	"bufio"
	"fmt"
	"os"
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
	Long: `Creates the configuration directory and an example config file at ~/.tasklog/config.yaml

Use --update to migrate an existing config file to the latest schema.
This will remove deprecated fields and add new optional fields.` + configHelp,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&updateConfig, "update", false, "Migrate existing config to latest schema (removes deprecated fields, adds new optional fields)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get config path (respects TASKLOG_CONFIG environment variable)
	configPath, err := config.GetConfigPath()
	if err != nil {
		fmt.Printf("failed to get config path: %v\n", err)
		return nil
	}

	// Ensure config directory exists
	dirErr := config.EnsureConfigDir()
	if dirErr != nil {
		fmt.Printf("failed to create config directory: %v\n", dirErr)
		return nil
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

	// Generate example config from the Config struct
	exampleData, err := config.GenerateExampleConfig()
	if err != nil {
		fmt.Printf("failed to generate example config: %v\n", err)
		return nil
	}

	// Write config file
	if err := os.WriteFile(configPath, exampleData, 0600); err != nil {
		fmt.Printf("failed to create config file: %v\n", err)
		return nil
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

// updateExistingConfig updates an existing config file using the migration logic
func updateExistingConfig(configPath string) error {
	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("failed to read config file: %v\n", err)
		return nil
	}

	// Run migration
	updatedData, summary, err := config.MigrateConfig(data)
	if err != nil {
		fmt.Printf("failed to migrate config: %v\n", err)
		return nil
	}

	// If nothing needs updating, inform the user
	if !summary.NeedsUpdate {
		fmt.Printf("✓ Config file is already up to date (version %d)!\n", summary.FromVersion)
		return nil
	}

	// Check if this is a version migration or optional sections update
	isVersionMigration := summary.FromVersion < summary.ToVersion
	hasMissingOptionalSections := len(summary.MissingOptionalSections) > 0

	// Show what will be changed
	if isVersionMigration {
		fmt.Printf("Config migration required: v%d → v%d\n", summary.FromVersion, summary.ToVersion)
		if summary.HasDeprecatedFields {
			fmt.Println("\n⚠️  Deprecated fields to be removed:")
			for _, field := range summary.DeprecatedFields {
				fmt.Printf("  - %s\n", field)
			}
		}
		if len(summary.MissingFields) > 0 {
			fmt.Println("\n✨ New required fields to be added:")
			for _, field := range summary.MissingFields {
				fmt.Printf("  - %s\n", field)
			}
		}
	}

	if hasMissingOptionalSections {
		if !isVersionMigration {
			fmt.Println("New optional configuration sections available:")
		}
		fmt.Println("\n✨ Optional sections to be added:")
		for _, section := range summary.MissingOptionalSections {
			fmt.Printf("  - %s (with example values)\n", section)
		}
	}

	// Ask for confirmation
	fmt.Printf("\nA backup will be created at: %s.backup\n", configPath)
	if isVersionMigration {
		fmt.Println("Note: Migration may reformat your YAML file.")
	}
	if !confirmUpdate("Do you want to proceed with the update?") {
		fmt.Println("Update cancelled.")
		return nil
	}

	// Create backup
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		fmt.Printf("failed to create backup: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Backup created at: %s\n", backupPath)

	// Apply optional sections if needed
	if hasMissingOptionalSections {
		updatedData, err = config.ApplyOptionalSections(updatedData, summary.MissingOptionalSections)
		if err != nil {
			fmt.Printf("failed to apply optional sections: %v\n", err)
			return nil
		}
	}

	// Write updated config
	if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
		fmt.Printf("failed to write updated config: %v\n", err)
		return nil
	}

	fmt.Println("✓ Config file updated successfully!")
	fmt.Println("\nChanges made:")
	if summary.HasDeprecatedFields {
		fmt.Println("  - Removed deprecated fields")
	}
	if len(summary.MissingFields) > 0 {
		fmt.Println("  - Added new required fields")
	}
	if hasMissingOptionalSections {
		fmt.Println("  - Added new optional sections with example values")
		fmt.Println("\nReview the config file and customize the new sections as needed.")
	}

	return nil
}

// confirmUpdate prompts the user for yes/no confirmation
// Accepts y, yes, Y, Yes, YES (case-insensitive)
// Returns false on any error or non-affirmative response
func confirmUpdate(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/N): ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
