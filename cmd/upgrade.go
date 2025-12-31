package cmd

import (
	"fmt"
	"os"

	"tasklog/internal/config"
	"tasklog/internal/ui"
	"tasklog/internal/updater"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/upgrade tasklog to the latest version",
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

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Manage tasklog upgrades",
	Long:  `Manage tasklog version upgrades.`,
}

var dismissCmd = &cobra.Command{
	Use:   "dismiss",
	Short: "Dismiss the current update notification",
	Long: `Dismiss the current update notification. You will see the notification again
after the next check interval (default: 24h), or immediately if a newer version
is released.`,
	Run: func(_ *cobra.Command, _ []string) {
		// Get config dir for cache
		configDir, err := config.GetConfigDir()
		if err != nil {
			Out.Error(fmt.Errorf("failed to get config directory: %w", err))
			os.Exit(1)
		}

		// Load config for check interval
		cfg, err := config.Load()
		if err != nil {
			Out.Error(fmt.Errorf("failed to load config: %w", err))
			os.Exit(1)
		}

		// Create updater
		upd := updater.NewUpdater(githubOwner, githubRepo, configDir, cfg.Update.CheckInterval)

		// Dismiss update
		if err := upd.DismissUpdate(); err != nil {
			Out.Error(err)
			os.Exit(1)
		}

		Out.Success("Update notification dismissed")
		Out.Printf("You'll be reminded again in %s, or immediately if a newer version is released.\n", cfg.Update.CheckInterval)
	},
}

func init() {
	upgradeCmd.AddCommand(installCmd)
	upgradeCmd.AddCommand(dismissCmd)
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Double-check this is an official build (shouldn't be reachable otherwise)
	if !IsOfficialBuild() {
		return fmt.Errorf("upgrade command is only available for official releases built by goreleaser\nBuild info: version=%s, builtBy=%s", version, builtBy)
	}

	Out.Println("🔍 Checking for updates...")

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

	// Get full update info for upgrade
	updateInfo, err := upd.GetUpdateInfo(version, cfg.Update.Channel)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if updateInfo == nil {
		Out.Success(fmt.Sprintf("You are already running the latest version (%s)", version))
		return nil
	}

	// Display update information
	Out.Println("\n📦 New version available!")
	Out.Printf("   Current version: %s\n", updateInfo.CurrentVersion)
	Out.Printf("   Latest version:  %s\n", updateInfo.LatestVersion)
	if updateInfo.IsPreRelease {
		Out.Printf("   Type:           Pre-release\n")
	}
	Out.Printf("   Release URL:     %s\n\n", updateInfo.ReleaseURL)

	if updateInfo.ReleaseNotes != "" {
		Out.Printf("Release notes:\n%s\n\n", updateInfo.ReleaseNotes)
	}

	// Confirm upgrade
	confirm, err := ui.Confirm("Do you want to upgrade now?")
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("upgrade cancelled by user")
	}

	// Download and replace binary
	Out.Println("\n📥 Installing new version...")

	// Perform upgrade
	backupPath, err := upd.InstallUpdate(updateInfo)
	if err != nil {
		if backupPath != "" {
			Out.Error(fmt.Errorf("upgrade failed: %w", err))
			Out.Warn("Attempting rollback...")

			// Restore from backup
			if restoreErr := upd.RollbackUpgrade(backupPath); restoreErr != nil {
				Out.Error(fmt.Errorf("rollback failed: %w", restoreErr))
				Out.Printf("Your backup is saved at: %s\n", backupPath)
				binaryPath, _ := os.Executable()
				Out.Printf("Please restore it manually: mv %s %s\n", backupPath, binaryPath)
				return fmt.Errorf("upgrade and rollback both failed")
			}

			Out.Success("Rollback successful. Your original version has been restored.")
		}
		return err
	}

	Out.Success(fmt.Sprintf("Successfully upgraded to version %s!", updateInfo.LatestVersion))
	Out.Printf("Backup saved at: %s\n", backupPath)
	Out.Println("\nYou can now run 'tasklog version' to verify the new version.")

	// Clear update cache after successful upgrade
	if clearErr := upd.ClearUpdateCache(); clearErr != nil {
		Out.Debug(fmt.Sprintf("Failed to clear update cache: %v", clearErr))
	}

	return nil
}
