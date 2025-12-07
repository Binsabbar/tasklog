package cmd

import (
	"fmt"
	"os"

	"tasklog/internal/config"
	"tasklog/internal/updater"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade tasklog to the latest version",
	Long: `Download and install the latest version of tasklog from GitHub releases.

This command will:
1. Check for the latest release (respects your update.channel config)
2. Download the appropriate binary for your OS/architecture
3. Create a backup of the current binary
4. Replace the current binary with the new version
5. Verify the upgrade was successful

The upgrade process is atomic - if anything fails, your current version remains intact.

Safety features:
- Automatic backup creation (.backup suffix)
- Checksum verification (if available)
- Permission checks before attempting upgrade
- Automatic rollback on failure

Release channels:
- If you're on a stable release (e.g., v1.0.0), you'll get stable updates
- If you're on a pre-release (e.g., v1.0.0-alpha.1), you'll get pre-release updates
- Configure update.channel in config to override: "", "stable", "alpha", "beta", "rc"

Note: If tasklog is installed in a system directory (e.g., /usr/local/bin),
you may need to run this command with sudo.` + configHelp,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Double-check this is an official build (shouldn't be reachable otherwise)
	if !IsOfficialBuild() {
		return fmt.Errorf("upgrade command is only available for official releases built by goreleaser\nBuild info: version=%s, builtBy=%s", version, builtBy)
	}

	fmt.Println("🔍 Checking for updates...")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		// If config doesn't exist, use empty channel (stable)
		cfg = &config.Config{}
	}

	// Get config dir for caching
	configDir, err := config.GetConfigDir()
	if err != nil {
		configDir = os.TempDir() // Fallback to temp dir if config dir unavailable
	}

	// Create updater
	upd := updater.NewUpdater(githubOwner, githubRepo, configDir, cfg.Update.CheckInterval)

	// Check for updates
	updateInfo, err := upd.CheckForUpdate(version, cfg.Update.Channel)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if updateInfo == nil {
		fmt.Printf("✓ You are already running the latest version (%s)\n", version)
		return nil
	}

	// Perform upgrade (handles user interaction and all upgrade logic)
	backupPath, err := upd.PerformUpgrade(updateInfo, updater.ConfirmAction)
	if err != nil {
		if backupPath != "" {
			fmt.Printf("\n❌ Upgrade failed: %v\n", err)
			fmt.Printf("\nAttempting rollback...\n")

			// Restore from backup
			if restoreErr := upd.RollbackUpgrade(backupPath); restoreErr != nil {
				fmt.Printf("❌ Rollback failed: %v\n", restoreErr)
				fmt.Printf("Your backup is saved at: %s\n", backupPath)
				binaryPath, _ := os.Executable()
				fmt.Printf("Please restore it manually: mv %s %s\n", backupPath, binaryPath)
				return fmt.Errorf("upgrade and rollback both failed")
			}

			fmt.Println("✓ Rollback successful. Your original version has been restored.")
		}
		return err
	}

	return nil
}
