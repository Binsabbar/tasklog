package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"tasklog/internal/config"
	"tasklog/internal/updater"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	upgradeChannel     string
	upgradeListChannel string
)

var installCmd = &cobra.Command{
	Use:   "install [version]",
	Short: "Install/upgrade tasklog to the latest or specified version",
	Long: `Download and install tasklog from GitHub releases.

This command will:
1. Check for the latest release or fetch the specified version
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

Usage examples:
  tasklog upgrade install                    # Upgrade to the best available version
  tasklog upgrade install v1.2.0             # Upgrade to specific version
  tasklog upgrade install --channel alpha    # Upgrade to latest alpha
  tasklog upgrade install --channel stable   # Upgrade to latest stable

Release channel priority:
- By default, prefers stable releases over pre-releases
- If you're on a pre-release, shows the best available update
- Use --channel to explicitly target a channel: stable, alpha, beta, rc

Note: If tasklog is installed in a system directory (e.g., /usr/local/bin),
you may need to run this command with sudo.` + configHelp,
	RunE: runUpgrade,
	Args: cobra.MaximumNArgs(1),
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Manage tasklog upgrades",
	Long:  `Manage tasklog version upgrades.`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available versions",
	Long: `List all available versions from GitHub releases.

Usage examples:
  tasklog upgrade list                 # List all versions
  tasklog upgrade list --channel stable # List only stable releases
  tasklog upgrade list --channel alpha  # List only alpha releases`,
	RunE: runListVersions,
}

var dismissCmd = &cobra.Command{
	Use:   "dismiss",
	Short: "Dismiss the current update notification for 24 hours",
	Long: `Dismiss the current update notification for 24 hours.

The notification will reappear after 24 hours, or immediately if a newer version
is released. This ensures you don't miss important stable releases while allowing
you to work without distractions.`,
	Run: func(_ *cobra.Command, _ []string) {
		// Get config dir for cache
		configDir, err := config.GetConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get config directory: %v\n", err)
			os.Exit(1)
		}

		// Load config for check interval
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
			os.Exit(1)
		}

		// Create updater
		upd := updater.NewUpdater(githubOwner, githubRepo, configDir, cfg.Update.CheckInterval)

		// Dismiss update
		if err := upd.DismissUpdate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✓ Update notification dismissed for 24 hours")
		fmt.Println("You'll be reminded tomorrow, or immediately if a newer version is released.")
	},
}

func init() {
	// Add flags to install command
	installCmd.Flags().StringVar(&upgradeChannel, "channel", "", "Release channel to upgrade to (stable, alpha, beta, rc)")

	// Add flags to list command
	listCmd.Flags().StringVar(&upgradeListChannel, "channel", "", "Filter by release channel (stable, alpha, beta, rc)")

	upgradeCmd.AddCommand(installCmd)
	upgradeCmd.AddCommand(listCmd)
	upgradeCmd.AddCommand(dismissCmd)
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Double-check this is an official build (shouldn't be reachable otherwise)
	if !IsOfficialBuild() {
		return fmt.Errorf("upgrade command is only available for official releases built by goreleaser\nBuild info: version=%s, builtBy=%s", version, builtBy)
	}

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

	var updateInfo *updater.UpdateInfo

	// Create context for the upgrade operations
	ctx := context.Background()

	// Check if user specified a version
	if len(args) > 0 {
		targetVersion := args[0]
		fmt.Printf("🔍 Fetching version %s...\n", targetVersion)

		updateInfo, err = upd.GetUpdateInfoForVersion(ctx, version, targetVersion)
		if err != nil {
			return fmt.Errorf("failed to fetch version %s: %w", targetVersion, err)
		}
	} else if upgradeChannel != "" {
		// User specified a channel
		fmt.Printf("🔍 Checking for latest %s release...\n", upgradeChannel)

		updateInfo, err = upd.GetUpdateInfo(ctx, version, upgradeChannel)
		if err != nil {
			if errors.Is(err, updater.ErrNoUpdateAvailable) {
				fmt.Printf("✓ You are already running the latest %s version (%s)\n", upgradeChannel, version)
				return nil
			}
			if errors.Is(err, updater.ErrDevBuild) {
				return fmt.Errorf("cannot upgrade development build")
			}
			return fmt.Errorf("failed to check for updates: %w", err)
		}
	} else {
		// Use best available update (smart channel detection)
		fmt.Println("🔍 Checking for the best available update...")

		updateInfo, err = upd.GetBestAvailableUpdate(ctx, version)
		if err != nil {
			if errors.Is(err, updater.ErrNoUpdateAvailable) {
				fmt.Printf("✓ You are already running the latest version (%s)\n", version)
				return nil
			}
			if errors.Is(err, updater.ErrDevBuild) {
				return fmt.Errorf("cannot upgrade development build")
			}
			return fmt.Errorf("failed to check for updates: %w", err)
		}
	}

	// Perform upgrade (handles user interaction and all upgrade logic)
	backupPath, err := upd.PerformUpgrade(ctx, updateInfo, confirmAction)
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

	fmt.Printf("\n✓ Successfully upgraded to version %s!\n", updateInfo.LatestVersion)
	fmt.Printf("Backup saved at: %s\n", backupPath)
	fmt.Println("\nYou can now run 'tasklog version' to verify the new version.")

	// Clear update cache after successful upgrade
	if clearErr := upd.ClearUpdateCache(); clearErr != nil {
		log.Debug().Err(clearErr).Msg("Failed to clear update cache")
	}

	return nil
}

func runListVersions(cmd *cobra.Command, args []string) error {
	// Double-check this is an official build
	if !IsOfficialBuild() {
		return fmt.Errorf("upgrade command is only available for official releases built by goreleaser\nBuild info: version=%s, builtBy=%s", version, builtBy)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}

	// Get config dir
	configDir, err := config.GetConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	// Create updater
	upd := updater.NewUpdater(githubOwner, githubRepo, configDir, cfg.Update.CheckInterval)

	// Fetch versions
	fmt.Println("🔍 Fetching available versions...")

	ctx := context.Background()
	versions, err := upd.ListAvailableVersions(ctx, upgradeListChannel)
	if err != nil {
		return fmt.Errorf("failed to fetch versions: %w", err)
	}

	if len(versions) == 0 {
		fmt.Println("No versions found")
		return nil
	}

	// Display versions
	fmt.Println("\nAvailable versions:")
	fmt.Println("─────────────────────────────────────────")

	currentPrefix := ""
	for _, v := range versions {
		marker := "  "
		if v.Version == version || "v"+version == v.Version {
			marker = "→ "
			currentPrefix = " (current)"
		} else {
			currentPrefix = ""
		}

		releaseType := ""
		if v.IsPreRelease {
			releaseType = fmt.Sprintf(" [%s]", v.Type)
		}

		fmt.Printf("%s%-20s%s%s\n", marker, v.Version, releaseType, currentPrefix)
	}

	fmt.Println("\nUsage:")
	fmt.Println("  tasklog upgrade install <version>  # Install specific version")
	fmt.Println("  tasklog upgrade install            # Install best available update")

	return nil
}

func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/N): ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
