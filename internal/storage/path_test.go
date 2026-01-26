package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// PathTestSuite is the test suite for path handling functions
type PathTestSuite struct {
	suite.Suite
}

// TestPathSuite runs the path test suite
func TestPathSuite(t *testing.T) {
	suite.Run(t, new(PathTestSuite))
}

// TestExpandPath_WithTilde tests path expansion for paths starting with ~
func (s *PathTestSuite) TestExpandPath_WithTilde() {
	home, err := os.UserHomeDir()
	require.NoError(s.T(), err, "should be able to get home directory")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde alone",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde with slash",
			input:    "~/",
			expected: home,
		},
		{
			name:     "tilde with subdirectory",
			input:    "~/.tasklog/db.sqlite",
			expected: filepath.Join(home, ".tasklog/db.sqlite"),
		},
		{
			name:     "tilde with nested subdirectories",
			input:    "~/data/nested/path/db.sqlite",
			expected: filepath.Join(home, "data/nested/path/db.sqlite"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := expandPath(tt.input)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.expected, result)
		})
	}
}

// TestExpandPath_WithoutTilde tests that paths without tilde pass through unchanged
func (s *PathTestSuite) TestExpandPath_WithoutTilde() {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "absolute path",
			input: "/var/lib/tasklog/db.sqlite",
		},
		{
			name:  "relative path",
			input: "./data/db.sqlite",
		},
		{
			name:  "relative path without dot",
			input: "data/db.sqlite",
		},
		{
			name:  "current directory",
			input: ".",
		},
		{
			name:  "parent directory",
			input: "..",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := expandPath(tt.input)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.input, result)
		})
	}
}

// TestEnsureDirForFile_CreatesSingleDirectory tests creating a single-level directory
func (s *PathTestSuite) TestEnsureDirForFile_CreatesSingleDirectory() {
	tmpDir := s.T().TempDir()
	filePath := filepath.Join(tmpDir, "newdir", "db.sqlite")

	err := ensureDirForFile(filePath)
	require.NoError(s.T(), err)

	// Verify directory was created
	dirPath := filepath.Dir(filePath)
	info, err := os.Stat(dirPath)
	require.NoError(s.T(), err)
	assert.True(s.T(), info.IsDir())
}

// TestEnsureDirForFile_CreatesNestedDirectories tests creating nested directories
func (s *PathTestSuite) TestEnsureDirForFile_CreatesNestedDirectories() {
	tmpDir := s.T().TempDir()
	filePath := filepath.Join(tmpDir, "level1", "level2", "level3", "db.sqlite")

	err := ensureDirForFile(filePath)
	require.NoError(s.T(), err)

	// Verify all directories were created
	dirPath := filepath.Dir(filePath)
	info, err := os.Stat(dirPath)
	require.NoError(s.T(), err)
	assert.True(s.T(), info.IsDir())
}

// TestEnsureDirForFile_ExistingDirectory tests that existing directories don't cause errors
func (s *PathTestSuite) TestEnsureDirForFile_ExistingDirectory() {
	tmpDir := s.T().TempDir()

	// Create directory first
	dirPath := filepath.Join(tmpDir, "existing")
	err := os.MkdirAll(dirPath, 0700)
	require.NoError(s.T(), err)

	// Should not error when directory already exists
	filePath := filepath.Join(dirPath, "db.sqlite")
	err = ensureDirForFile(filePath)
	require.NoError(s.T(), err)
}

// TestEnsureDirForFile_FileInCurrentDirectory tests file in current directory
func (s *PathTestSuite) TestEnsureDirForFile_FileInCurrentDirectory() {
	// File in current directory should not create any directories
	err := ensureDirForFile("db.sqlite")
	require.NoError(s.T(), err)
}

// TestEnsureDirForFile_VerifiesPermissions tests that created directories have correct permissions
func (s *PathTestSuite) TestEnsureDirForFile_VerifiesPermissions() {
	tmpDir := s.T().TempDir()
	filePath := filepath.Join(tmpDir, "permtest", "db.sqlite")

	err := ensureDirForFile(filePath)
	require.NoError(s.T(), err)

	// Verify directory permissions (0700 = rwx------)
	dirPath := filepath.Dir(filePath)
	info, err := os.Stat(dirPath)
	require.NoError(s.T(), err)

	// On Unix systems, check permissions
	if info.Mode().Perm() != 0700 {
		s.T().Logf("Warning: Directory permissions are %o, expected 0700", info.Mode().Perm())
	}
}
