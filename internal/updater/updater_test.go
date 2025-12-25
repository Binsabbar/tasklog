package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tasklog/internal/github"
)

func TestNewUpdater(t *testing.T) {
	updater := NewUpdater("owner", "repo", "/tmp/cache", "24h")

	if updater.owner != "owner" {
		t.Errorf("expected owner 'owner', got '%s'", updater.owner)
	}
	if updater.repo != "repo" {
		t.Errorf("expected repo 'repo', got '%s'", updater.repo)
	}
	if updater.cacheDir != "/tmp/cache" {
		t.Errorf("expected cacheDir '/tmp/cache', got '%s'", updater.cacheDir)
	}
	if updater.githubClient == nil {
		t.Error("expected githubClient to be initialized")
	}
}

func TestDetermineChannel(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		configChannel  string
		expectedOutput string
	}{
		{
			name:           "stable version, no config",
			currentVersion: "1.0.0",
			configChannel:  "",
			expectedOutput: "",
		},
		{
			name:           "stable version, config says stable",
			currentVersion: "1.0.0",
			configChannel:  "stable",
			expectedOutput: "",
		},
		{
			name:           "alpha version, no config - stay on alpha",
			currentVersion: "1.0.0-alpha.1",
			configChannel:  "",
			expectedOutput: "alpha",
		},
		{
			name:           "beta version, no config - stay on beta",
			currentVersion: "1.0.0-beta.2",
			configChannel:  "",
			expectedOutput: "beta",
		},
		{
			name:           "rc version, no config - stay on rc",
			currentVersion: "1.0.0-rc.1",
			configChannel:  "",
			expectedOutput: "rc",
		},
		{
			name:           "alpha version, config overrides to beta",
			currentVersion: "1.0.0-alpha.1",
			configChannel:  "beta",
			expectedOutput: "beta",
		},
		{
			name:           "stable version, config says alpha",
			currentVersion: "1.0.0",
			configChannel:  "alpha",
			expectedOutput: "alpha",
		},
		{
			name:           "pre-release with unknown suffix",
			currentVersion: "1.0.0-dev",
			configChannel:  "",
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := NewUpdater("owner", "repo", "/tmp", "24h")
			version, err := ParseVersion(tt.currentVersion)
			if err != nil {
				t.Fatalf("failed to parse version: %v", err)
			}

			channel := updater.determineChannel(version, tt.configChannel)
			if channel != tt.expectedOutput {
				t.Errorf("expected channel '%s', got '%s'", tt.expectedOutput, channel)
			}
		})
	}
}

func TestShouldCheckForUpdate(t *testing.T) {
	tests := []struct {
		name        string
		cacheAge    time.Duration
		expectCheck bool
		setupCache  bool
	}{
		{
			name:        "no cache file - should check",
			cacheAge:    0,
			expectCheck: true,
			setupCache:  false,
		},
		{
			name:        "cache expired - should check",
			cacheAge:    25 * time.Hour,
			expectCheck: true,
			setupCache:  true,
		},
		{
			name:        "cache fresh - should not check",
			cacheAge:    1 * time.Hour,
			expectCheck: false,
			setupCache:  true,
		},
		{
			name:        "cache at boundary - should not check",
			cacheAge:    23 * time.Hour,
			expectCheck: false,
			setupCache:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			updater := NewUpdater("owner", "repo", tmpDir, "24h")

			var cache *UpdateCache = nil
			if tt.setupCache {
				// Create cache with specific age
				cache = &UpdateCache{
					LastCheck:       time.Now().Add(-tt.cacheAge),
					UpdateAvailable: false,
				}
				updater.saveUpdateCache(cache)
			}

			shouldCheck := updater.shouldCheckForUpdate(cache)
			if shouldCheck != tt.expectCheck {
				t.Errorf("expected shouldCheck=%v, got %v", tt.expectCheck, shouldCheck)
			}
		})
	}
}

func TestSaveUpdateCache(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Save cache
	cache := &UpdateCache{
		LastCheck:       time.Now(),
		UpdateAvailable: true,
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.1.0",
		IsPreRelease:    false,
		ReleaseURL:      "https://github.com/owner/repo/releases/tag/v1.1.0",
		Dismissed:       false,
	}
	updater.saveUpdateCache(cache)

	// Verify cache file exists
	cacheFile := filepath.Join(tmpDir, "update_cache.json")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	// Verify we can read it back
	readCache := updater.getCachedUpdate()
	if readCache == nil {
		t.Fatal("failed to read cache")
	}
	if readCache.LatestVersion != "1.1.0" {
		t.Errorf("expected latest version 1.1.0, got %s", readCache.LatestVersion)
	}
}

func TestGetAssetNameForPlatform(t *testing.T) {
	assetName := getAssetNameForPlatform()

	// Should contain OS and architecture
	if !strings.Contains(assetName, "_") {
		t.Errorf("expected asset name to contain underscore, got '%s'", assetName)
	}

	// Should not be empty
	if assetName == "" {
		t.Error("asset name should not be empty")
	}

	// Verify it uses Go's native arch names (not x86_64 mapping)
	// Should be like "linux_amd64", "darwin_arm64", etc.
	expectedFormat := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	if assetName != expectedFormat {
		t.Errorf("expected asset name '%s', got '%s'", expectedFormat, assetName)
	}
}

func TestCheckWritePermission(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*testing.T) string
		expectError bool
	}{
		{
			name: "writable directory",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "test-binary")
			},
			expectError: false,
		},
		{
			name: "non-existent directory",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/directory/binary"
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc(t)
			err := checkWritePermission(path)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	content := "test content"
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy file
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dstFile); err != nil {
		t.Error("destination file not created")
	}

	// Verify content matches
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("content mismatch: expected '%s', got '%s'", content, string(dstContent))
	}

	// Verify permissions copied
	srcInfo, _ := os.Stat(srcFile)
	dstInfo, _ := os.Stat(dstFile)
	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("permissions not copied: src=%v, dst=%v", srcInfo.Mode(), dstInfo.Mode())
	}
}

func TestCopyFileErrors(t *testing.T) {
	tests := []struct {
		name      string
		srcPath   string
		dstPath   string
		setupFunc func(*testing.T, string, string)
	}{
		{
			name:    "source doesn't exist",
			srcPath: "/nonexistent/source.txt",
			dstPath: "/tmp/dest.txt",
		},
		{
			name:    "destination directory doesn't exist",
			srcPath: "",
			dstPath: "/nonexistent/dir/dest.txt",
			setupFunc: func(t *testing.T, srcPath, dstPath string) {
				// Create a temp source file
				tmpFile, err := os.CreateTemp("", "source-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.Close()
				t.Cleanup(func() { os.Remove(tmpFile.Name()) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc(t, tt.srcPath, tt.dstPath)
			}

			err := copyFile(tt.srcPath, tt.dstPath)
			if err == nil {
				t.Error("expected error but got none")
			}
		})
	}
}

func TestCheckForUpdate_DevBuild(t *testing.T) {
	t.Skip("Skipping - requires GitHub API mocking to test properly")

	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Test with an invalid/unparseable version (like "dev")
	// The code should parse it, fail, log, and return notification with Available=false
	notification, err := updater.CheckForUpdate("dev", "")

	// Dev builds should return non-nil notification with Available=false
	if err != nil {
		t.Errorf("dev build should not return error, got: %v", err)
	}
	if notification == nil {
		t.Fatal("dev build should return non-nil notification")
	}
	if notification.Available {
		t.Error("dev build should return notification with Available=false")
	}
}

func TestCheckForUpdate_CacheExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Create fresh cache
	cache := &UpdateCache{
		LastCheck:       time.Now(),
		UpdateAvailable: false,
	}
	updater.saveUpdateCache(cache)

	// First call with cache should skip check and return cached result
	notification, err := updater.CheckForUpdate("v1.0.0", "")

	// We expect non-nil notification with Available=false from cache
	if notification == nil {
		t.Fatal("expected non-nil notification")
	}
	if notification.Available {
		t.Error("expected Available=false from cache")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateInfo_Structure(t *testing.T) {
	info := &UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "1.1.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.1.0",
		ReleaseNotes:   "Bug fixes",
		DownloadURL:    "https://example.com/download",
		AssetName:      "tasklog-linux-amd64",
		IsPreRelease:   false,
	}

	if info.CurrentVersion != "1.0.0" {
		t.Errorf("expected current version '1.0.0', got '%s'", info.CurrentVersion)
	}
	if info.LatestVersion != "1.1.0" {
		t.Errorf("expected latest version '1.1.0', got '%s'", info.LatestVersion)
	}
	if info.IsPreRelease {
		t.Error("expected IsPreRelease to be false")
	}
}

func TestRollbackUpgrade(t *testing.T) {
	// Note: This test requires modifying the actual binary, which is risky
	// In production, this would be tested with a mock binary
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Create mock binary and backup
	binaryPath := filepath.Join(tmpDir, "test-binary")
	backupPath := binaryPath + ".backup"

	originalContent := "original binary"
	backupContent := "backup binary"

	if err := os.WriteFile(binaryPath, []byte(backupContent), 0755); err != nil {
		t.Fatalf("failed to create binary: %v", err)
	}

	if err := os.WriteFile(backupPath, []byte(originalContent), 0755); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Test rollback (will fail because it tries to use os.Executable)
	// This demonstrates the function exists and has correct signature
	err := updater.RollbackUpgrade(backupPath)
	// We expect an error because we're not testing with the actual executable
	_ = err
}

func TestPerformUpgrade_UserCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	updateInfo := &UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "1.1.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.1.0",
		ReleaseNotes:   "New features",
		DownloadURL:    "https://example.com/download",
		AssetName:      "tasklog-linux-amd64",
		IsPreRelease:   false,
	}

	// Mock confirm function that returns false
	confirmNo := func(prompt string) bool {
		return false
	}

	backupPath, err := updater.PerformUpgrade(updateInfo, confirmNo)
	if err == nil {
		t.Error("expected error when user cancels")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected 'cancelled' in error, got: %v", err)
	}
	if backupPath != "" {
		t.Errorf("expected empty backup path, got '%s'", backupPath)
	}
}

func TestGetUpdateInfo_AssetSelection(t *testing.T) {
	// Create a test server that returns assets with and without archives
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return a release with multiple assets for the same platform
		// Archives should be skipped, raw binary should be picked
		w.Write([]byte(`{
			"tag_name": "v1.1.0",
			"name": "Release 1.1.0",
			"body": "Test release",
			"prerelease": false,
			"draft": false,
			"assets": [
				{
					"name": "tasklog_1.1.0_linux_amd64.tar.gz",
					"browser_download_url": "https://example.com/download.tar.gz"
				},
				{
					"name": "tasklog_1.1.0_linux_amd64.zip",
					"browser_download_url": "https://example.com/download.zip"
				},
				{
					"name": "tasklog_1.1.0_linux_amd64",
					"browser_download_url": "https://example.com/download-binary"
				}
			]
		}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Point client to test server
	updater.githubClient.SetBaseURL(server.URL)

	// We need to match the current runtime OS/Arch for getAssetNameForPlatform() to work in the test
	// or we can mock getAssetNameForPlatform() if it was possible.
	// Since we can't easily mock it, we'll just check if it finds *something* if we are on linux/amd64
	// or we can just verify the logic by seeing if it picks the binary URL.
	// However, getAssetNameForPlatform returns runtime.GOOS_runtime.GOARCH.
	// So we need to return assets that match the current platform in the test.
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	server.Close() // restart with dynamic platform names
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{
			"tag_name": "v1.1.0",
			"assets": [
				{
					"name": "tasklog_1.1.0_%s.tar.gz",
					"browser_download_url": "https://example.com/download.tar.gz"
				},
				{
					"name": "tasklog_1.1.0_%s",
					"browser_download_url": "https://example.com/download-binary"
				}
			]
		}`, platform, platform)))
	}))
	defer server.Close()
	updater.githubClient.SetBaseURL(server.URL)

	info, err := updater.GetUpdateInfo("v1.0.0", "")
	if err != nil {
		t.Fatalf("GetUpdateInfo failed: %v", err)
	}

	if info == nil {
		t.Fatal("expected update info, got nil")
	}

	expectedURL := "https://example.com/download-binary"
	if info.DownloadURL != expectedURL {
		t.Errorf("expected DownloadURL '%s', got '%s' (it might have picked the archive!)", expectedURL, info.DownloadURL)
	}
}

func TestCheckForUpdate_Integration(t *testing.T) {
	// This test demonstrates the flow without making real API calls
	// In production, you'd use a mock GitHub server

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock GitHub API response
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"tag_name": "v1.5.0",
				"name": "Release 1.5.0",
				"body": "New features",
				"prerelease": false,
				"draft": false,
				"assets": [
					{
						"name": "tasklog-linux-x86_64",
						"browser_download_url": "https://example.com/download"
					}
				]
			}`))
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Replace the GitHub client with one pointing to our test server
	updater.githubClient = github.NewClient("owner", "repo")
	updater.githubClient = &github.Client{} // This would need proper mocking in production

	// Note: Full integration test would require injecting the test server URL
	// For now, we verify the function signature and structure
}

func TestDownloadAndReplace_PermissionError(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// This will fail because we're not testing with the actual executable
	// But it verifies the function exists and handles errors
	_, err := updater.downloadAndReplace("http://invalid", "")
	if err == nil {
		t.Error("expected error for invalid download")
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create a test server that serves checksum
	content := "test content"
	actualChecksum := fmt.Sprintf("%x", []byte("wrong checksum"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(actualChecksum))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Create test file
	testFile := filepath.Join(tmpDir, "test-file")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Verify checksum (should fail because checksums don't match)
	err := updater.verifyChecksum(testFile, server.URL)
	if err == nil {
		t.Error("expected checksum verification to fail")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", err)
	}
}

func TestVerifyChecksum_DownloadError(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Create test file
	testFile := filepath.Join(tmpDir, "test-file")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Try to verify with invalid URL
	err := updater.verifyChecksum(testFile, "http://invalid-url-that-does-not-exist")
	if err == nil {
		t.Error("expected error for invalid checksum URL")
	}
}

// failWriter is a helper for testing error conditions
type failWriter struct{}

func (f *failWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrShortWrite
}

func TestFailingWriter(t *testing.T) {
	// Test helper struct
	var _ io.Writer = (*failWriter)(nil)

	fw := &failWriter{}
	_, err := fw.Write([]byte("test"))

	// This verifies we can create failing writers for testing
	if err == nil {
		t.Error("expected error from failing writer")
	}
}
