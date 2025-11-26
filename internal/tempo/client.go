package tempo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a Tempo API client
type Client struct {
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a new Tempo API client
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WorklogRequest represents a request to create a worklog in Tempo
type WorklogRequest struct {
	IssueID          string             `json:"issueId"` // Numeric issue ID (required in v4)
	TimeSpentSeconds int                `json:"timeSpentSeconds"`
	StartDate        string             `json:"startDate"` // Format: YYYY-MM-DD
	StartTime        string             `json:"startTime"` // Format: HH:MM:SS
	Description      string             `json:"description,omitempty"`
	AuthorAccountID  string             `json:"authorAccountId"` // Required in v4
	Attributes       []WorklogAttribute `json:"attributes,omitempty"`
}

// WorklogAttribute represents a Tempo worklog attribute (for labels)
type WorklogAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WorklogResponse represents the response from Tempo after creating a worklog
type WorklogResponse struct {
	TempoWorklogID   int    `json:"tempoWorklogId"`
	JiraWorklogID    int    `json:"jiraWorklogId"`
	IssueKey         string `json:"issueKey"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	StartDate        string `json:"startDate"`
	StartTime        string `json:"startTime"`
	Description      string `json:"description"`
	CreatedAt        string `json:"createdAt"`
	Author           struct {
		AccountID string `json:"accountId"`
	} `json:"author"`
}

// AddWorklog adds a worklog entry to Tempo
func (c *Client) AddWorklog(issueID, authorAccountID string, timeSpentSeconds int, started time.Time, label, description string) (*WorklogResponse, error) {
	log.Debug().
		Str("issue_id", issueID).
		Int("seconds", timeSpentSeconds).
		Str("label", label).
		Msg("Adding worklog to Tempo")

	// Use Tempo API v4 endpoint
	endpoint := "https://api.tempo.io/4/worklogs"

	// Format date and time for Tempo
	startDate := started.Format("2006-01-02")
	startTime := started.Format("15:04:05")

	payload := WorklogRequest{
		IssueID:          issueID,
		AuthorAccountID:  authorAccountID,
		TimeSpentSeconds: timeSpentSeconds,
		StartDate:        startDate,
		StartTime:        startTime,
		Description:      description,
	}

	// Add label as an attribute if provided
	// Note: The attribute key depends on your Tempo configuration
	// You may need to adjust this based on your Tempo setup
	if label != "" {
		// Construct description with label
		if payload.Description != "" {
			payload.Description = fmt.Sprintf("[%s] %s", label, payload.Description)
		} else {
			payload.Description = fmt.Sprintf("[%s]", label)
		}
	}

	var response WorklogResponse
	if err := c.doRequest("POST", endpoint, payload, &response); err != nil {
		return nil, fmt.Errorf("failed to add worklog to Tempo: %w", err)
	}

	log.Info().
		Str("issue_id", issueID).
		Int("tempo_id", response.TempoWorklogID).
		Str("time", formatSeconds(timeSpentSeconds)).
		Msg("Worklog added to Tempo successfully")

	return &response, nil
}

// GetWorklogs retrieves worklogs for a date range
func (c *Client) GetWorklogs(from, to time.Time, authorAccountID string) ([]WorklogResponse, error) {
	log.Debug().
		Str("from", from.Format("2006-01-02")).
		Str("to", to.Format("2006-01-02")).
		Str("author", authorAccountID).
		Msg("Fetching worklogs from Tempo")

	// Use Tempo API v4 endpoint with author filter
	endpoint := fmt.Sprintf(
		"https://api.tempo.io/4/worklogs?from=%s&to=%s&author=%s",
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
		authorAccountID,
	)

	var response struct {
		Results []WorklogResponse `json:"results"`
	}

	if err := c.doRequest("GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch worklogs from Tempo: %w", err)
	}

	// Filter by author client-side as an extra safeguard
	filtered := []WorklogResponse{}
	for _, wl := range response.Results {
		if wl.Author.AccountID == authorAccountID {
			filtered = append(filtered, wl)
		}
	}

	log.Debug().
		Int("total", len(response.Results)).
		Int("filtered", len(filtered)).
		Msg("Retrieved worklogs from Tempo")

	return filtered, nil
}

// GetTodayWorklogs retrieves today's worklogs for a specific author
func (c *Client) GetTodayWorklogs(authorAccountID string) ([]WorklogResponse, error) {
	today := time.Now()
	return c.GetWorklogs(today, today, authorAccountID)
}

// doRequest performs an HTTP request to the Tempo API
func (c *Client) doRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = strings.NewReader(string(jsonData))
		log.Debug().Str("body", string(jsonData)).Msg("Request body")
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("Tempo API request failed")
		return fmt.Errorf("Tempo API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// formatSeconds formats seconds into human-readable time
func formatSeconds(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}
