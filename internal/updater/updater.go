package updater

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"tasklog/internal/github"

	"github.com/rs/zerolog/log"
	str2duration "github.com/xhit/go-str2duration/v2"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	ReleaseNotes   string
	DownloadURL    string
	AssetName      string
	IsPreRelease   bool
}

// Updater handles checking for updates and upgrading binaries
type Updater struct {
	owner         string
	repo          string
	githubClient  *github.Client
	cacheDir      string
	checkInterval time.Duration // How often to check for updates
}

// NewUpdater creates a new updater
// checkInterval is a duration string like "24h", "1d", "2h30m"
func NewUpdater(owner, repo, cacheDir, checkInterval string) *Updater {
	// Parse check interval, default to 24h if invalid
	interval, err := str2duration.ParseDuration(checkInterval)
	if err != nil {
		log.Debug().Str("interval", checkInterval).Msg("Invalid check interval, using default 24h")
		interval = 24 * time.Hour
	}

	return &Updater{
		owner:         owner,
		repo:          repo,
		githubClient:  github.NewClient(owner, repo),
		cacheDir:      cacheDir,
		checkInterval: interval,
	}
}

// CheckForUpdate checks if a new version is available
// channel can be "", "alpha", "beta", or "rc" for pre-releases
// Returns UpdateInfo if update is available, nil if up-to-date, error on failure
func (u *Updater) CheckForUpdate(currentVersion, channel string) (*UpdateInfo, error) {
	// Check cache first to avoid hitting GitHub API frequently
	if !u.shouldCheckForUpdate() {
		log.Debug().Msg("Skipping update check (cache not expired)")
		return nil, nil
	}

	// Parse current version
	current, err := ParseVersion(currentVersion)
	if err != nil {
		log.Debug().Str("version", currentVersion).Msg("Failed to parse current version (probably dev build)")
		return nil, nil // Don't error on dev builds
	}

	// Determine which channel to check based on current version and config
	effectiveChannel := u.determineChannel(current, channel)

	// Fetch latest release from GitHub
	var release *github.Release
	if effectiveChannel == "" {
		// Check for stable releases only
		release, err = u.githubClient.GetLatestRelease()
	} else {
		// Check for pre-releases
		release, err = u.githubClient.GetLatestPreRelease(effectiveChannel)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Update cache timestamp
	u.updateCacheTimestamp()

	// Parse latest version
	latest, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version %s: %w", release.TagName, err)
	}

	// Check if update is available
	if !latest.IsNewerThan(current) {
		log.Debug().
			Str("current", current.String()).
			Str("latest", latest.String()).
			Msg("No update available")
		return nil, nil
	}

	// Find the appropriate binary asset for current platform
	assetName := getAssetNameForPlatform()
	downloadURL := ""
	actualAssetName := ""

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.BrowserDownloadURL
			actualAssetName = asset.Name
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return &UpdateInfo{
		CurrentVersion: current.String(),
		LatestVersion:  latest.String(),
		ReleaseURL:     u.githubClient.GetReleaseURL(release.TagName),
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
		AssetName:      actualAssetName,
		IsPreRelease:   release.Prerelease,
	}, nil
}

// PerformUpgrade downloads and installs the new version
// Returns backup path and error
func (u *Updater) PerformUpgrade(updateInfo *UpdateInfo, confirm func(string) bool) (string, error) {
	// Display update information
	fmt.Printf("\n📦 New version available!\n")
	fmt.Printf("   Current version: %s\n", updateInfo.CurrentVersion)
	fmt.Printf("   Latest version:  %s\n", updateInfo.LatestVersion)
	if updateInfo.IsPreRelease {
		fmt.Printf("   Type:           Pre-release\n")
	}
	fmt.Printf("   Release URL:     %s\n\n", updateInfo.ReleaseURL)

	if updateInfo.ReleaseNotes != "" {
		fmt.Printf("Release notes:\n%s\n\n", updateInfo.ReleaseNotes)
	}

	// Confirm upgrade
	if !confirm("Do you want to upgrade now?") {
		return "", fmt.Errorf("upgrade cancelled by user")
	}

	// Download and replace binary
	fmt.Println("\n📥 Downloading new version...")

	backupPath, err := u.downloadAndReplace(updateInfo.DownloadURL, "")
	if err != nil {
		return backupPath, err
	}

	fmt.Printf("\n✓ Successfully upgraded to version %s!\n", updateInfo.LatestVersion)
	fmt.Printf("Backup saved at: %s\n", backupPath)
	fmt.Println("\nYou can now run 'tasklog version' to verify the new version.")

	return backupPath, nil
}

// downloadAndReplace downloads the new binary and replaces the current one atomically
func (u *Updater) downloadAndReplace(downloadURL, checksumURL string) (string, error) {
	// Get current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Resolve symlinks
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve binary path: %w", err)
	}

	log.Info().Str("path", binaryPath).Msg("Current binary path")

	// Check if we have write permission
	if err := checkWritePermission(binaryPath); err != nil {
		return "", fmt.Errorf("insufficient permissions to update binary: %w\nTry running with sudo or install to a user-writable location", err)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "tasklog-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Download new binary
	log.Info().Str("url", downloadURL).Msg("Downloading new version")
	if err := u.githubClient.DownloadAsset(downloadURL, tmpFile); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	tmpFile.Close()

	// Verify checksum if provided
	if checksumURL != "" {
		log.Debug().Msg("Verifying checksum")
		if err := u.verifyChecksum(tmpPath, checksumURL); err != nil {
			return "", fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Make new binary executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Create backup of current binary
	backupPath := binaryPath + ".backup"
	log.Info().Str("backup", backupPath).Msg("Creating backup")
	if err := copyFile(binaryPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	// Atomic replace: rename temp file to binary path
	log.Info().Msg("Replacing binary")
	if err := os.Rename(tmpPath, binaryPath); err != nil {
		return backupPath, fmt.Errorf("failed to replace binary: %w", err)
	}

	log.Info().Msg("Update completed successfully!")
	return backupPath, nil
}

// RollbackUpgrade restores from backup
func (u *Updater) RollbackUpgrade(backupPath string) error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	if err := os.Rename(backupPath, binaryPath); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	return nil
}

// determineChannel determines which release channel to check
// If user is on pre-release, continue checking that channel unless config overrides
// If user is on stable, check stable unless config specifies pre-release
func (u *Updater) determineChannel(currentVersion *Version, configChannel string) string {
	// If config explicitly sets a channel, use it
	if configChannel != "" && configChannel != "stable" {
		return configChannel
	}

	// If config says stable or empty, and current version is pre-release, stay on pre-release channel
	if currentVersion.Prerelease() != "" {
		// Extract the channel from pre-release (e.g., "alpha.1" -> "alpha")
		parts := strings.Split(currentVersion.Prerelease(), ".")
		if len(parts) > 0 {
			channel := parts[0]
			// Validate it's a known channel
			if channel == "alpha" || channel == "beta" || channel == "rc" {
				return channel
			}
		}
	}

	// Default to stable (empty channel)
	return ""
}

// shouldCheckForUpdate checks if we should check for updates based on cache
func (u *Updater) shouldCheckForUpdate() bool {
	cacheFile := filepath.Join(u.cacheDir, "update_check_timestamp")

	info, err := os.Stat(cacheFile)
	if err != nil {
		return true // Cache doesn't exist, should check
	}

	return time.Since(info.ModTime()) > u.checkInterval
}

// updateCacheTimestamp updates the cache timestamp file
func (u *Updater) updateCacheTimestamp() {
	cacheFile := filepath.Join(u.cacheDir, "update_check_timestamp")

	// Ensure cache directory exists
	os.MkdirAll(u.cacheDir, 0755)

	// Touch the file to update timestamp
	f, err := os.OpenFile(cacheFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to update cache timestamp")
		return
	}
	f.Close()
}

// verifyChecksum verifies the SHA256 checksum of the downloaded file
func (u *Updater) verifyChecksum(filePath, checksumURL string) error {
	// Download checksum
	tmpFile, err := os.CreateTemp("", "tasklog-checksum-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for checksum: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := u.githubClient.DownloadAsset(checksumURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}

	// Read checksum
	tmpFile.Seek(0, 0)
	checksumData, err := io.ReadAll(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to read checksum: %w", err)
	}

	expectedChecksum := strings.TrimSpace(string(checksumData))

	// Calculate actual checksum
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualChecksum := fmt.Sprintf("%x", h.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// ConfirmAction prompts the user for yes/no confirmation
func ConfirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/N): ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// getAssetNameForPlatform returns the expected asset name for the current platform
func getAssetNameForPlatform() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go's GOARCH to common naming conventions
	arch := goarch
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}

	// Common patterns: tasklog_darwin_x86_64, tasklog-darwin-arm64, etc.
	return fmt.Sprintf("%s_%s", goos, arch)
}

// checkWritePermission checks if we can write to the given path
func checkWritePermission(path string) error {
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".tasklog_write_test")

	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}
