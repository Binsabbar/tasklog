package cmd

import (
	"fmt"
	"os"
	"strings"

	"tasklog/internal/config"
	"tasklog/internal/prerelease"
	"tasklog/internal/updater"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	githubOwner = "Binsabbar"
	githubRepo  = "tasklog"
)

const configHelp = `

Configuration:
  Default config location: ~/.tasklog/config.yaml
  Override with environment variable: TASKLOG_CONFIG=/path/to/config.yaml`

var rootCmd = &cobra.Command{
	Use:   "tasklog",
	Short: "Interactive time tracking tool with Jira and Tempo integration",
	Long: `Tasklog is an interactive CLI tool for tracking time on Jira tasks.
It integrates with Jira Cloud API and Tempo to help you log time efficiently.` + configHelp,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Check for pre-release config issues first (only for pre-release builds)
		if IsPreReleaseBuild() {
			checkPreReleaseConfigIssues()
		}

		// Check for updates before every command (synchronous to ensure notification shows)
		// Skip if not an official build
		if !IsOfficialBuild() {
			log.Debug().Msg("Skipping update check (not an official release build)")
			return
		}
		checkForUpdates()
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		log.Error().Err(err).Msg("Failed to ensure config directory")
	}
}

// checkForUpdates checks for updates
func checkForUpdates() {
	// Load config to check if updates are enabled
	cfg, err := config.Load()
	if err != nil {
		// If config doesn't exist or fails to load, skip update check
		return
	}

	// Check if update checking is disabled
	// Default is false (checks enabled) unless explicitly set to true
	if cfg.Update.Disabled {
		return
	}

	// Get config dir for caching
	configDir, err := config.GetConfigDir()
	if err != nil {
		return // Skip if we can't get config dir
	}

	// Check for updates (handles cache internally)
	upd := updater.NewUpdater(githubOwner, githubRepo, configDir, cfg.Update.CheckInterval)
	notification, err := upd.CheckForUpdate(version, cfg.Update.Channel)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to check for updates")
		return
	}

	// Display notification if update is available
	if notification.Available {
		// Skip if user dismissed this update
		if notification.Dismissed {
			return
		}

		preReleaseTag := ""
		if notification.IsPreRelease {
			preReleaseTag = " (pre-release)"
		}
		fmt.Fprintf(os.Stderr, "\n📦 New version available: %s → %s%s\n", notification.CurrentVersion, notification.LatestVersion, preReleaseTag)
		fmt.Fprintf(os.Stderr, "   ⬆️  Run 'tasklog upgrade install' to update\n")
		fmt.Fprintf(os.Stderr, "   📄 Release notes: %s\n", notification.ReleaseURL)
		fmt.Fprintf(os.Stderr, "   ⚠️  To dismiss this notification: 'tasklog upgrade dismiss'\n\n")
	}
}

func checkConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)

		configPath := os.Getenv("TASKLOG_CONFIG")
		if configPath == "" {
			configPath = "~/.tasklog/config.yaml"
		}

		fmt.Fprintf(os.Stderr, "Please create a config file at %s\n", configPath)
		fmt.Fprintf(os.Stderr, "Or set TASKLOG_CONFIG environment variable to specify a custom location.\n")
		fmt.Fprintf(os.Stderr, "See config.example.yaml for an example configuration.\n")
		return nil, err
	}
	return cfg, nil
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func SetVersionInfo(v, c, d, b string) {
	version = v
	commit = c
	date = d
	builtBy = b
	// Update rootCmd version string after variables are set
	rootCmd.Version = GetVersion()
}

func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, date)
}

// IsOfficialBuild returns true if the binary was built by goreleaser (official release)
func IsOfficialBuild() bool {
	return builtBy == "goreleaser"
}

// IsPreReleaseBuild returns true if this is a pre-release version (alpha, beta, rc)
func IsPreReleaseBuild() bool {
	// Check if version contains alpha, beta, or rc
	return strings.Contains(version, "alpha") || strings.Contains(version, "beta") || strings.Contains(version, "rc")
}

func SetCommandsVisibility() {
	upgradeCmd.Hidden = !IsOfficialBuild()
}

// checkPreReleaseConfigIssues checks for configuration issues in pre-release builds
func checkPreReleaseConfigIssues() {
	// Try to load config file
	configPath, err := config.GetConfigPath()
	if err != nil {
		return // No config path, skip check
	}

	// Read config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return // Can't read config, skip check
	}

	// Validate config for known pre-release issues
	issues, err := prerelease.ValidateConfig(configData)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to validate pre-release config")
		return
	}

	// Display issues if found
	if len(issues) > 0 {
		fmt.Fprint(os.Stderr, prerelease.FormatIssues(issues))
	}
}
