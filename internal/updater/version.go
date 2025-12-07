package updater

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
)

// Version wraps hashicorp/go-version for semantic versioning
type Version struct {
	version *version.Version
}

// ParseVersion parses a semantic version string (e.g., "v1.2.3", "1.2.3-beta", "v1.2.3+build123")
func ParseVersion(v string) (*Version, error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	if v == "dev" || v == "" {
		// Development version - treat as 0.0.0-dev
		v = "0.0.0-dev"
	}

	// Parse using hashicorp/go-version
	parsed, err := version.NewVersion(v)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	return &Version{
		version: parsed,
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	return v.version.String()
}

// IsNewerThan returns true if v is newer than other
// Pre-release versions are considered older than release versions
func (v *Version) IsNewerThan(other *Version) bool {
	return v.version.GreaterThan(other.version)
}

// Equals returns true if v equals other (ignoring build metadata)
func (v *Version) Equals(other *Version) bool {
	return v.version.Equal(other.version)
}

// Prerelease returns the pre-release version (e.g., "beta.1" from "1.2.3-beta.1")
func (v *Version) Prerelease() string {
	return v.version.Prerelease()
}
