package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	apiURL         = "https://api.github.com/repos/%s/%s/releases/latest"
	releasesURL    = "https://api.github.com/repos/%s/%s/releases"
	releaseWebURL  = "https://github.com/%s/%s/releases/tag/%s"
	defaultTimeout = 10 * time.Second
)

// Release represents a GitHub release
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Client handles GitHub API interactions
type Client struct {
	owner      string
	repo       string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client
func NewClient(owner, repo string) *Client {
	return &Client{
		owner:      owner,
		repo:       repo,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// GetLatestRelease fetches the latest stable release
func (c *Client) GetLatestRelease() (*Release, error) {
	url := fmt.Sprintf(apiURL, c.owner, c.repo)
	body, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var release Release
	if err := json.NewDecoder(body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &release, nil
}

// GetLatestPreRelease fetches the latest release (including pre-releases)
func (c *Client) GetLatestPreRelease(channel string) (*Release, error) {
	// Fetch all releases
	url := fmt.Sprintf(releasesURL, c.owner, c.repo)
	body, err := c.doGetRequest(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var releases []Release
	if err := json.NewDecoder(body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter for the latest non-draft release matching the channel
	for _, release := range releases {
		if release.Draft {
			continue
		}

		switch {
		case channel == "" && release.Prerelease:
			continue
		case channel != "" && release.Prerelease:
			if matchesChannel(release.TagName, channel) {
				return &release, nil
			}
		case channel == "" && !release.Prerelease:
			return &release, nil
		}
	}

	return nil, fmt.Errorf("no release found for channel: %s", channel)
}

// GetReleaseURL returns the web URL for a release
func (c *Client) GetReleaseURL(tagName string) string {
	return fmt.Sprintf(releaseWebURL, c.owner, c.repo, tagName)
}

// DownloadAsset downloads an asset from a URL
func (c *Client) DownloadAsset(url string, out io.Writer) error {
	body, err := c.doGetRequest(url)
	if err != nil {
		return err
	}
	defer body.Close()

	_, err = io.Copy(out, body)
	return err
}

// matchesChannel checks if a tag name matches the given pre-release channel
func matchesChannel(tagName, channel string) bool {
	// Simple matching: check if tag contains the channel name
	// Examples:
	// v1.0.0-alpha.1 matches "alpha"
	// v1.0.0-beta matches "beta"
	// v1.0.0-rc.1 matches "rc"

	// Remove v prefix if present
	if len(tagName) > 0 && tagName[0] == 'v' {
		tagName = tagName[1:]
	}

	// Check if tag contains the channel after a dash
	return strings.Contains(tagName, "-"+channel)
}

func (c *Client) doGetRequest(url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}
