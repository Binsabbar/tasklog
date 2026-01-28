package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

// Sentinel errors for update operations
var (
	// ErrDevBuild indicates the current version is a development build and cannot check for updates
	ErrDevBuild = fmt.Errorf("development build cannot check for updates")
	// ErrNoUpdateAvailable indicates no newer version is available
	ErrNoUpdateAvailable = fmt.Errorf("no update available")
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

// UpdateNotification contains information to display update notification
type UpdateNotification struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
	IsPreRelease   bool
	ReleaseURL     string
	Dismissed      bool // User dismissed this update
}

// UpdateCache stores cached update information
type UpdateCache struct {
	LastCheck       time.Time `json:"last_check"`
	UpdateAvailable bool      `json:"update_available"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	IsPreRelease    bool      `json:"is_prerelease"`
	ReleaseURL      string    `json:"release_url"`
	Dismissed       bool      `json:"dismissed"`
	DismissedAt     time.Time `json:"dismissed_at"` // When user dismissed the notification
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

// CheckForBestUpdate checks for the best available update across all channels
// Uses smart detection to prefer stable over pre-release
// Returns UpdateNotification with availability info
func (u *Updater) CheckForBestUpdate(ctx context.Context, currentVersion string) (*UpdateNotification, error) {
	// First check cache
	cache := u.getCachedUpdate()
	if cache != nil && !u.shouldCheckForUpdate(cache) {
		// Check if dismissed - but only respect dismissal for 24 hours max
		if cache.Dismissed {
			dismissDuration := time.Since(cache.DismissedAt)
			if dismissDuration < 24*time.Hour {
				// Still within 24h dismiss window
				return &UpdateNotification{
					Available:      cache.UpdateAvailable,
					CurrentVersion: cache.CurrentVersion,
					LatestVersion:  cache.LatestVersion,
					IsPreRelease:   cache.IsPreRelease,
					ReleaseURL:     cache.ReleaseURL,
					Dismissed:      true,
				}, nil
			}
			// Dismiss expired, fall through to check GitHub
		} else {
			// Not dismissed, return cached notification
			return &UpdateNotification{
				Available:      cache.UpdateAvailable,
				CurrentVersion: cache.CurrentVersion,
				LatestVersion:  cache.LatestVersion,
				IsPreRelease:   cache.IsPreRelease,
				ReleaseURL:     cache.ReleaseURL,
				Dismissed:      false,
			}, nil
		}
	}

	// Use GetBestAvailableUpdate to find the best version
	updateInfo, err := u.GetBestAvailableUpdate(ctx, currentVersion)
	if err != nil {
		// Handle known errors
		if errors.Is(err, ErrNoUpdateAvailable) {
			// No update available - save cache and return notification
			current, _ := ParseVersion(currentVersion)
			if current == nil {
				return &UpdateNotification{Available: false}, nil
			}
			u.saveUpdateCache(&UpdateCache{
				LastCheck:       time.Now(),
				UpdateAvailable: false,
				CurrentVersion:  current.String(),
				LatestVersion:   current.String(),
			})
			return &UpdateNotification{
				Available:      false,
				CurrentVersion: current.String(),
				LatestVersion:  current.String(),
			}, nil
		}
		if errors.Is(err, ErrDevBuild) {
			// Dev build - no update check possible
			return &UpdateNotification{Available: false}, nil
		}
		// Other error
		return nil, fmt.Errorf("failed to fetch best update: %w", err)
	}

	// Save cache with update available (dismissed = false on fresh check)
	u.saveUpdateCache(&UpdateCache{
		LastCheck:       time.Now(),
		UpdateAvailable: true,
		CurrentVersion:  updateInfo.CurrentVersion,
		LatestVersion:   updateInfo.LatestVersion,
		IsPreRelease:    updateInfo.IsPreRelease,
		ReleaseURL:      updateInfo.ReleaseURL,
		Dismissed:       false,
	})

	return &UpdateNotification{
		Available:      true,
		CurrentVersion: updateInfo.CurrentVersion,
		LatestVersion:  updateInfo.LatestVersion,
		IsPreRelease:   updateInfo.IsPreRelease,
		ReleaseURL:     updateInfo.ReleaseURL,
		Dismissed:      false,
	}, nil
}

// CheckForUpdate checks if a new version is available
// channel can be "", "alpha", "beta", or "rc" for pre-releases
// Returns UpdateNotification with availability info, always returns non-nil notification
func (u *Updater) CheckForUpdate(ctx context.Context, currentVersion, channel string) (*UpdateNotification, error) {
	// First check cache for existing notification
	cache := u.getCachedUpdate()
	if cache != nil && !u.shouldCheckForUpdate(cache) {
		// Return cached notification (including dismissed status)
		return &UpdateNotification{
			Available:      cache.UpdateAvailable,
			CurrentVersion: cache.CurrentVersion,
			LatestVersion:  cache.LatestVersion,
			IsPreRelease:   cache.IsPreRelease,
			ReleaseURL:     cache.ReleaseURL,
			Dismissed:      cache.Dismissed,
		}, nil
	}

	// Parse current version
	current, err := ParseVersion(currentVersion)
	if err != nil {
		log.Debug().Str("version", currentVersion).Err(err).Msg("Failed to parse current version (probably dev build)")
		// Return notification indicating no update (dev build)
		return &UpdateNotification{Available: false}, nil
	}

	// Determine which channel to check based on current version and config
	effectiveChannel := u.determineChannel(current, channel)

	// Fetch latest release from GitHub
	var release *github.Release
	if effectiveChannel == "" {
		// Check for stable releases only
		release, err = u.githubClient.GetLatestRelease(ctx)
	} else {
		// Check for pre-releases
		release, err = u.githubClient.GetLatestPreRelease(ctx, effectiveChannel)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

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
		// Save cache indicating no update available
		u.saveUpdateCache(&UpdateCache{
			LastCheck:       time.Now(),
			UpdateAvailable: false,
			CurrentVersion:  current.String(),
			LatestVersion:   latest.String(),
		})
		return &UpdateNotification{
			Available:      false,
			CurrentVersion: current.String(),
			LatestVersion:  latest.String(),
		}, nil
	}

	// Update is available
	// Always reset dismissed flag when we re-check GitHub (after check interval):
	// - If this is a newer version: show notification
	// - If same version was dismissed: show reminder (interval passed)
	u.saveUpdateCache(&UpdateCache{
		LastCheck:       time.Now(),
		UpdateAvailable: true,
		CurrentVersion:  current.String(),
		LatestVersion:   latest.String(),
		IsPreRelease:    release.Prerelease,
		ReleaseURL:      u.githubClient.GetReleaseURL(release.TagName),
		Dismissed:       false, // Always false after check interval
	})

	return &UpdateNotification{
		Available:      true,
		CurrentVersion: current.String(),
		LatestVersion:  latest.String(),
		IsPreRelease:   release.Prerelease,
		ReleaseURL:     u.githubClient.GetReleaseURL(release.TagName),
	}, nil
}

// GetUpdateInfo fetches full update info including download URLs for upgrade
// Returns UpdateInfo if update is available, nil if up-to-date, error on failure
func (u *Updater) GetUpdateInfo(ctx context.Context, currentVersion, channel string) (*UpdateInfo, error) {
	// Parse current version
	current, err := ParseVersion(currentVersion)
	if err != nil {
		log.Debug().Str("version", currentVersion).Err(err).Msg("Failed to parse current version (probably dev build)")
		return nil, ErrDevBuild
	}

	// Determine which channel to check based on current version and config
	effectiveChannel := u.determineChannel(current, channel)

	// Fetch latest release from GitHub
	var release *github.Release
	if effectiveChannel == "" {
		// Check for stable releases only
		release, err = u.githubClient.GetLatestRelease(ctx)
	} else {
		// Check for pre-releases
		release, err = u.githubClient.GetLatestPreRelease(ctx, effectiveChannel)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

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
		return nil, ErrNoUpdateAvailable
	}

	// Find the appropriate binary asset for current platform
	assetName := getAssetNameForPlatform()
	downloadURL := ""
	actualAssetName := ""

	for _, asset := range release.Assets {
		// Skip archives as we need a raw binary for atomic replacement.
		// The updater currently doesn't implement extraction logic.
		if strings.HasSuffix(asset.Name, ".tar.gz") ||
			strings.HasSuffix(asset.Name, ".zip") ||
			strings.HasSuffix(asset.Name, ".tgz") {
			continue
		}

		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.BrowserDownloadURL
			actualAssetName = asset.Name
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	u.saveUpdateCache(&UpdateCache{
		LastCheck:       time.Now(),
		UpdateAvailable: true,
		CurrentVersion:  current.String(),
		LatestVersion:   latest.String(),
		IsPreRelease:    release.Prerelease,
		ReleaseURL:      u.githubClient.GetReleaseURL(release.TagName),
		Dismissed:       false,
	})

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

// GetUpdateInfoForVersion fetches update info for a specific version
// Returns UpdateInfo if version exists, error otherwise
func (u *Updater) GetUpdateInfoForVersion(ctx context.Context, currentVersion, targetVersion string) (*UpdateInfo, error) {
	// Parse current version
	current, err := ParseVersion(currentVersion)
	if err != nil {
		log.Debug().Str("version", currentVersion).Err(err).Msg("Failed to parse current version (probably dev build)")
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	// Fetch the specific release
	release, err := u.githubClient.GetReleaseByTag(ctx, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release %s: %w", targetVersion, err)
	}

	// Parse target version
	target, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target version %s: %w", release.TagName, err)
	}

	// Find the appropriate binary asset for current platform
	assetName := getAssetNameForPlatform()
	downloadURL := ""
	actualAssetName := ""

	for _, asset := range release.Assets {
		// Skip archives
		if strings.HasSuffix(asset.Name, ".tar.gz") ||
			strings.HasSuffix(asset.Name, ".zip") ||
			strings.HasSuffix(asset.Name, ".tgz") {
			continue
		}

		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.BrowserDownloadURL
			actualAssetName = asset.Name
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary found for platform %s/%s in release %s", runtime.GOOS, runtime.GOARCH, targetVersion)
	}

	return &UpdateInfo{
		CurrentVersion: current.String(),
		LatestVersion:  target.String(),
		ReleaseURL:     u.githubClient.GetReleaseURL(release.TagName),
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
		AssetName:      actualAssetName,
		IsPreRelease:   release.Prerelease,
	}, nil
}

// ListAvailableVersions lists all available versions, optionally filtered by channel
// channel can be "", "stable", "alpha", "beta", or "rc"
func (u *Updater) ListAvailableVersions(ctx context.Context, channel string) ([]ReleaseInfo, error) {
	releases, err := u.githubClient.GetAllReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	var versions []ReleaseInfo
	for _, release := range releases {
		// Parse version
		version, err := ParseVersion(release.TagName)
		if err != nil {
			log.Debug().Str("tag", release.TagName).Msg("Skipping release with unparseable version")
			continue
		}

		// Filter by channel
		if channel == "stable" && release.Prerelease {
			continue
		}
		if channel != "" && channel != "stable" && release.Prerelease {
			if !matchesChannel(release.TagName, channel) {
				continue
			}
		}
		if channel != "" && channel != "stable" && !release.Prerelease {
			continue // Only show pre-releases when channel is specified
		}

		releaseType := "stable"
		if release.Prerelease {
			// Extract channel from version
			if pre := version.Prerelease(); pre != "" {
				parts := strings.Split(pre, ".")
				if len(parts) > 0 {
					releaseType = parts[0]
				}
			}
		}

		versions = append(versions, ReleaseInfo{
			Version:     release.TagName,
			Type:        releaseType,
			IsPreRelease: release.Prerelease,
			ReleaseURL:  u.githubClient.GetReleaseURL(release.TagName),
		})
	}

	return versions, nil
}

// ReleaseInfo contains summary information about a release
type ReleaseInfo struct {
	Version      string
	Type         string // "stable", "alpha", "beta", "rc"
	IsPreRelease bool
	ReleaseURL   string
}

// matchesChannel checks if a version tag matches the given channel
func matchesChannel(tagName, channel string) bool {
	return strings.Contains(strings.ToLower(tagName), "-"+strings.ToLower(channel))
}

// PerformUpgrade downloads and installs the new version
// Returns backup path and error
func (u *Updater) PerformUpgrade(ctx context.Context, updateInfo *UpdateInfo, confirm func(string) bool) (string, error) {
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

	backupPath, err := u.downloadAndReplace(ctx, updateInfo.DownloadURL, "")
	if err != nil {
		return backupPath, err
	}

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

// ClearUpdateCache removes the update cache file
func (u *Updater) ClearUpdateCache() error {
	cacheFile := u.getCacheFilePath()
	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	return nil
}

// DismissUpdate marks the current update notification as dismissed for 24 hours
func (u *Updater) DismissUpdate() error {
	cache := u.getCachedUpdate()
	if cache == nil || !cache.UpdateAvailable {
		return fmt.Errorf("no update available to dismiss")
	}

	// Mark as dismissed with timestamp (max 24h)
	cache.Dismissed = true
	cache.DismissedAt = time.Now()
	u.saveUpdateCache(cache)
	return nil
}

// determineChannel determines which release channel to check
// New logic: Be smarter about showing updates across channels
func (u *Updater) determineChannel(currentVersion *Version, configChannel string) string {
	// If config explicitly sets a channel, use it
	if configChannel != "" && configChannel != "stable" {
		return configChannel
	}

	// If config explicitly says stable, return stable
	if configChannel == "stable" {
		return ""
	}

	// If no config preference and user is on pre-release, extract channel
	// This maintains backward compatibility for users who want to stay on pre-release
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

// GetBestAvailableUpdate checks all relevant channels and returns the best update
// For users on stable releases: ONLY returns stable updates (never pre-releases)
// For users on pre-releases: Returns best available (prefers stable > rc > beta > alpha)
// This ensures stable users aren't surprised by alpha/beta notifications
func (u *Updater) GetBestAvailableUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error) {
	// Parse current version
	current, err := ParseVersion(currentVersion)
	if err != nil {
		log.Debug().Str("version", currentVersion).Err(err).Msg("Failed to parse current version (probably dev build)")
		return nil, ErrDevBuild
	}

	// Fetch all releases
	releases, err := u.githubClient.GetAllReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	// Find the best available update
	var bestRelease *github.Release
	var bestVersion *Version
	currentIsStable := current.Prerelease() == ""

	for i := range releases {
		release := &releases[i]
		version, err := ParseVersion(release.TagName)
		if err != nil {
			continue
		}

		// Skip if not newer than current
		if !version.IsNewerThan(current) {
			continue
		}

		// CRITICAL: If user is on a stable release, NEVER promote pre-releases
		// Users on stable should only see stable updates (unless they use --channel explicitly)
		if currentIsStable && release.Prerelease {
			continue
		}

		// If this is the first valid newer version, use it
		if bestVersion == nil {
			bestVersion = version
			bestRelease = release
			continue
		}

		// Prefer stable over pre-release
		if !release.Prerelease && bestRelease.Prerelease {
			bestVersion = version
			bestRelease = release
			continue
		}

		// If both are stable or both are pre-release, pick the newer one
		if release.Prerelease == bestRelease.Prerelease && version.IsNewerThan(bestVersion) {
			bestVersion = version
			bestRelease = release
		}
	}

	if bestRelease == nil {
		return nil, ErrNoUpdateAvailable
	}

	// Find the appropriate binary asset
	assetName := getAssetNameForPlatform()
	downloadURL := ""
	actualAssetName := ""

	for _, asset := range bestRelease.Assets {
		if strings.HasSuffix(asset.Name, ".tar.gz") ||
			strings.HasSuffix(asset.Name, ".zip") ||
			strings.HasSuffix(asset.Name, ".tgz") {
			continue
		}

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
		LatestVersion:  bestVersion.String(),
		ReleaseURL:     u.githubClient.GetReleaseURL(bestRelease.TagName),
		ReleaseNotes:   bestRelease.Body,
		DownloadURL:    downloadURL,
		AssetName:      actualAssetName,
		IsPreRelease:   bestRelease.Prerelease,
	}, nil
}

// downloadAndReplace downloads the new binary and replaces the current one atomically
func (u *Updater) downloadAndReplace(ctx context.Context, downloadURL, checksumURL string) (string, error) {
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
	if err := u.githubClient.DownloadAsset(ctx, downloadURL, tmpFile); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	_ = tmpFile.Close()

	// Verify checksum if provided
	if checksumURL != "" {
		log.Debug().Msg("Verifying checksum")
		if err := u.verifyChecksum(ctx, tmpPath, checksumURL); err != nil {
			return "", fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Make new binary executable
	if err := os.Chmod(tmpPath, 0o755); err != nil { //nolint:gosec // G302: binary needs to be executable
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

// verifyChecksum verifies the SHA256 checksum of the downloaded file
func (u *Updater) verifyChecksum(ctx context.Context, filePath, checksumURL string) error {
	// Download checksum
	tmpFile, err := os.CreateTemp("", "tasklog-checksum-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for checksum: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := u.githubClient.DownloadAsset(ctx, checksumURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}

	// Read checksum
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek checksum file: %w", err)
	}
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

// getCachedUpdate returns cached update info if available and not expired
func (u *Updater) getCachedUpdate() *UpdateCache {
	cacheFile := u.getCacheFilePath()

	// Read cache file
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil // No cache file
	}

	// Unmarshal cache
	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		log.Debug().Err(err).Msg("Failed to unmarshal cache")
		return nil
	}

	// Return cache (caller should check if update is available and not dismissed)
	return &cache
}

// ShowUpdateNotification checks cache and displays update notification if available
func (u *Updater) getCacheFilePath() string {
	return filepath.Join(u.cacheDir, "update_cache.json")
}

// shouldCheckForUpdate checks if we should check for updates based on cache
func (u *Updater) shouldCheckForUpdate(cache *UpdateCache) bool {
	if cache == nil {
		return true // No cache, should check
	}

	return time.Since(cache.LastCheck) > u.checkInterval
}

// saveUpdateCache saves the update cache to disk
func (u *Updater) saveUpdateCache(cache *UpdateCache) {
	cacheFile := u.getCacheFilePath()

	// Ensure cache directory exists
	if err := os.MkdirAll(u.cacheDir, 0o755); err != nil { //nolint:gosec // G301: standard directory permissions
		log.Debug().Err(err).Msg("Failed to create cache directory")
		return
	}

	// Marshal cache to JSON
	data, err := json.Marshal(cache)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to marshal cache")
		return
	}

	// Write cache file
	if err := os.WriteFile(cacheFile, data, 0o644); err != nil { //nolint:gosec // G302: standard file permissions
		log.Debug().Err(err).Msg("Failed to write cache file")
	}
}

// utils
// ConfirmAction prompts the user for yes/no confirmation
// getAssetNameForPlatform returns the expected asset name for the current platform
func getAssetNameForPlatform() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Use Go's native arch names (matches goreleaser's {{ .Os }}_{{ .Arch }} template)
	// Pattern: tasklog_1.0.0_linux_amd64, tasklog_1.0.0_darwin_arm64
	return fmt.Sprintf("%s_%s", goos, goarch)
}

// checkWritePermission checks if we can write to the given path
func checkWritePermission(path string) error {
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".tasklog_write_test")

	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	_ = f.Close()
	_ = os.Remove(testFile)
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
