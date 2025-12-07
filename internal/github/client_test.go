package github

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("owner", "repo")

	if client.owner != "owner" {
		t.Errorf("expected owner to be 'owner', got '%s'", client.owner)
	}
	if client.repo != "repo" {
		t.Errorf("expected repo to be 'repo', got '%s'", client.repo)
	}
	if client.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}
}

func TestGetLatestRelease(t *testing.T) {
	t.Skip("Skipping - would require mocking internal URLs or dependency injection")
	// This test would need the ability to override GitHub API URLs for testing
	// In production, consider adding a baseURL field to Client for testing
}

func TestGetReleaseURL(t *testing.T) {
	client := NewClient("testowner", "testrepo")
	url := client.GetReleaseURL("v1.2.3")

	expected := "https://github.com/testowner/testrepo/releases/tag/v1.2.3"
	if url != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, url)
	}
}

func TestDownloadAsset(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectError    bool
		expectedOutput string
	}{
		{
			name:           "successful download",
			responseStatus: http.StatusOK,
			responseBody:   "binary content here",
			expectError:    false,
			expectedOutput: "binary content here",
		},
		{
			name:           "404 not found",
			responseStatus: http.StatusNotFound,
			responseBody:   "",
			expectError:    true,
			expectedOutput: "",
		},
		{
			name:           "500 server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "",
			expectError:    true,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient("owner", "repo")
			client.httpClient = server.Client()

			var buf bytes.Buffer
			err := client.DownloadAsset(server.URL, &buf)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if buf.String() != tt.expectedOutput {
				t.Errorf("expected output '%s', got '%s'", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestMatchesChannel(t *testing.T) {
	tests := []struct {
		name     string
		tagName  string
		channel  string
		expected bool
	}{
		{
			name:     "alpha matches",
			tagName:  "v1.0.0-alpha.1",
			channel:  "alpha",
			expected: true,
		},
		{
			name:     "beta matches",
			tagName:  "v1.0.0-beta",
			channel:  "beta",
			expected: true,
		},
		{
			name:     "rc matches",
			tagName:  "v1.0.0-rc.1",
			channel:  "rc",
			expected: true,
		},
		{
			name:     "alpha doesn't match beta",
			tagName:  "v1.0.0-alpha.1",
			channel:  "beta",
			expected: false,
		},
		{
			name:     "stable doesn't match alpha",
			tagName:  "v1.0.0",
			channel:  "alpha",
			expected: false,
		},
		{
			name:     "tag without v prefix",
			tagName:  "1.0.0-alpha.1",
			channel:  "alpha",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesChannel(tt.tagName, tt.channel)
			if result != tt.expected {
				t.Errorf("matchesChannel(%s, %s) = %v, expected %v",
					tt.tagName, tt.channel, result, tt.expected)
			}
		})
	}
}

func TestGetLatestReleaseHTTPError(t *testing.T) {
	t.Skip("Skipping - would require mocking internal URLs")
	// Real-world testing would be done via integration tests
}

func TestDownloadAssetInvalidWriter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	client := NewClient("owner", "repo")

	// Use a writer that always fails
	failWriter := &failingWriter{}
	err := client.DownloadAsset(server.URL, failWriter)
	if err == nil {
		t.Error("expected error from failing writer, got none")
	}
}

// failingWriter is a test writer that always returns an error
type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrShortWrite
}
