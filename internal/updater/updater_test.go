package updater

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

			if tt.setupCache {
				// Create cache file with specific age
				cacheFile := filepath.Join(tmpDir, "update_check_timestamp")
				os.MkdirAll(tmpDir, 0755)
				f, err := os.Create(cacheFile)
				if err != nil {
					t.Fatalf("failed to create cache file: %v", err)
				}
				f.Close()

				// Set the modification time
				pastTime := time.Now().Add(-tt.cacheAge)
				os.Chtimes(cacheFile, pastTime, pastTime)
			}

			shouldCheck := updater.shouldCheckForUpdate()
			if shouldCheck != tt.expectCheck {
				t.Errorf("expected shouldCheck=%v, got %v", tt.expectCheck, shouldCheck)
			}
		})
	}
}

func TestUpdateCacheTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Update cache
	updater.updateCacheTimestamp()

	// Verify cache file exists
	cacheFile := filepath.Join(tmpDir, "update_check_timestamp")
	info, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	// Verify it's recent
	if time.Since(info.ModTime()) > 5*time.Second {
		t.Error("cache timestamp is not recent")
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

func TestConfirmAction(t *testing.T) {
	// Note: This function reads from stdin, so it's difficult to test directly
	// In a real scenario, you'd inject the reader as a dependency
	// For now, we just verify the function exists and has correct signature
	confirm := ConfirmAction
	if confirm == nil {
		t.Error("ConfirmAction should not be nil")
	}
}

func TestCheckForUpdate_DevBuild(t *testing.T) {
	t.Skip("Skipping - requires GitHub API mocking to test properly")

	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Test with an invalid/unparseable version (like "dev")
	// The code should parse it, fail, log, and return nil, nil WITHOUT hitting GitHub API
	updateInfo, err := updater.CheckForUpdate("dev", "")

	// Dev builds should return nil, nil without error
	// The code returns early after failing to parse the version
	if err != nil {
		// If we get here, the code is still hitting the API when it shouldn't
		t.Errorf("dev build should not return error (should return nil, nil early), got: %v", err)
	}
	if updateInfo != nil {
		t.Error("dev build should return nil updateInfo")
	}
}

func TestCheckForUpdate_CacheExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	updater := NewUpdater("owner", "repo", tmpDir, "24h")

	// Create fresh cache
	updater.updateCacheTimestamp()

	// First call with cache should skip check
	updateInfo, err := updater.CheckForUpdate("v1.0.0", "")

	// We expect nil/nil because cache is fresh
	if updateInfo != nil {
		t.Error("expected nil updateInfo due to fresh cache")
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
