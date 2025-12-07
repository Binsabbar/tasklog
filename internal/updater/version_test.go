package updater

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStr    string
		wantPrerel string
		wantErr    bool
	}{
		{
			name:    "simple version",
			input:   "1.2.3",
			wantStr: "1.2.3",
		},
		{
			name:    "version with v prefix",
			input:   "v1.2.3",
			wantStr: "1.2.3",
		},
		{
			name:       "version with pre-release",
			input:      "1.2.3-beta",
			wantStr:    "1.2.3-beta",
			wantPrerel: "beta",
		},
		{
			name:    "version with build metadata",
			input:   "1.2.3+build123",
			wantStr: "1.2.3+build123",
		},
		{
			name:       "version with pre-release and build",
			input:      "v1.2.3-alpha.1+build456",
			wantStr:    "1.2.3-alpha.1+build456",
			wantPrerel: "alpha.1",
		},
		{
			name:       "dev version",
			input:      "dev",
			wantStr:    "0.0.0-dev",
			wantPrerel: "dev",
		},
		{
			name:    "version missing patch",
			input:   "1.2",
			wantStr: "1.2.0",
		},
		{
			name:    "invalid format - non-numeric",
			input:   "1.a.3",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVersion() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseVersion() unexpected error: %v", err)
				return
			}
			if got.String() != tt.wantStr {
				t.Errorf("ParseVersion().String() = %v, want %v", got.String(), tt.wantStr)
			}
			if got.Prerelease() != tt.wantPrerel {
				t.Errorf("ParseVersion().Prerelease() = %v, want %v", got.Prerelease(), tt.wantPrerel)
			}
		})
	}
}

func TestVersionIsNewerThan(t *testing.T) {
	tests := []struct {
		name  string
		v1    string
		v2    string
		newer bool
	}{
		{"major version newer", "2.0.0", "1.9.9", true},
		{"minor version newer", "1.2.0", "1.1.9", true},
		{"patch version newer", "1.1.2", "1.1.1", true},
		{"same version", "1.2.3", "1.2.3", false},
		{"older major", "1.0.0", "2.0.0", false},
		{"release newer than pre-release", "1.2.3", "1.2.3-beta", true},
		{"pre-release older than release", "1.2.3-alpha", "1.2.3", false},
		{"same pre-release", "1.2.3-beta", "1.2.3-beta", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := ParseVersion(tt.v1)
			if err != nil {
				t.Fatalf("failed to parse v1: %v", err)
			}
			v2, err := ParseVersion(tt.v2)
			if err != nil {
				t.Fatalf("failed to parse v2: %v", err)
			}

			got := v1.IsNewerThan(v2)
			if got != tt.newer {
				t.Errorf("Version(%s).IsNewerThan(%s) = %v, want %v", tt.v1, tt.v2, got, tt.newer)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			want:  "1.2.3",
		},
		{
			name:  "with pre-release",
			input: "1.2.3-beta",
			want:  "1.2.3-beta",
		},
		{
			name:  "with build metadata",
			input: "1.2.3+build123",
			want:  "1.2.3+build123",
		},
		{
			name:  "with pre-release and build",
			input: "1.2.3-alpha+build456",
			want:  "1.2.3-alpha+build456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := ParseVersion(tt.input)
			if err != nil {
				t.Fatalf("failed to parse version: %v", err)
			}
			got := version.String()
			if got != tt.want {
				t.Errorf("Version.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
