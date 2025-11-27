package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client represents a Jira API client
type Client struct {
	baseURL    string
	username   string
	apiToken   string
	projectKey string
	httpClient *http.Client
}

// NewClient creates a new Jira API client
func NewClient(baseURL, username, apiToken, projectKey string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		username:   username,
		apiToken:   apiToken,
		projectKey: projectKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Issue represents a Jira issue
type Issue struct {
	ID     string      `json:"id"` // Numeric ID as string
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssueFields represents Jira issue fields
type IssueFields struct {
	Summary  string       `json:"summary"`
	Status   IssueStatus  `json:"status"`
	Assignee *IssueUser   `json:"assignee"`
	Worklog  *WorklogList `json:"worklog,omitempty"`
}

// WorklogList represents the worklog field in issue response
type WorklogList struct {
	Worklogs []Worklog `json:"worklogs"`
}

// IssueStatus represents Jira issue status
type IssueStatus struct {
	Name string `json:"name"`
}

// IssueUser represents a Jira user
type IssueUser struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// SearchResult represents Jira search results
type SearchResult struct {
	Issues []Issue `json:"issues"`
	Total  int     `json:"total"`
}

// Worklog represents a Jira worklog entry
type Worklog struct {
	ID               string          `json:"id,omitempty"`
	IssueID          string          `json:"issueId,omitempty"`
	TimeSpent        string          `json:"timeSpent"`
	TimeSpentSeconds int             `json:"timeSpentSeconds"`
	Started          string          `json:"started"` // Format: 2024-11-11T10:00:00.000+0000
	Comment          json.RawMessage `json:"comment,omitempty"`
	Author           *IssueUser      `json:"author,omitempty"`
}

// GetInProgressIssues retrieves issues in progress for the current user
// The statuses parameter allows filtering by multiple status values (e.g., ["In Progress", "In Review"])
func (c *Client) GetInProgressIssues(statuses []string) ([]Issue, error) {
	log.Debug().Msg("Fetching in-progress issues")

	// Default to "In Progress" if no statuses provided
	if len(statuses) == 0 {
		statuses = []string{"In Progress"}
	}

	// Build status filter
	var statusFilter string
	if len(statuses) == 1 {
		statusFilter = fmt.Sprintf("status = '%s'", statuses[0])
	} else {
		statusFilters := make([]string, len(statuses))
		for i, status := range statuses {
			statusFilters[i] = fmt.Sprintf("'%s'", status)
		}
		statusFilter = fmt.Sprintf("status IN (%s)", strings.Join(statusFilters, ", "))
	}

	jql := fmt.Sprintf("assignee = currentUser() AND %s", statusFilter)
	if c.projectKey != "" {
		jql = fmt.Sprintf("%s AND project = %s", jql, c.projectKey)
	}
	jql = fmt.Sprintf("%s ORDER BY updated DESC", jql)

	// Use POST method with JSON body as recommended by Jira API v3
	endpoint := fmt.Sprintf("%s/rest/api/3/search/jql", c.baseURL)

	payload := map[string]interface{}{
		"jql":        jql,
		"fields":     []string{"summary", "status", "assignee"},
		"maxResults": 50,
	}

	var result SearchResult
	if err := c.doRequest("POST", endpoint, payload, &result); err != nil {
		return nil, fmt.Errorf("failed to fetch in-progress issues: %w", err)
	}

	log.Debug().Int("count", len(result.Issues)).Msg("Retrieved in-progress issues")
	return result.Issues, nil
}

// GetIssue retrieves a specific issue by key
func (c *Client) GetIssue(issueKey string) (*Issue, error) {
	log.Debug().Str("key", issueKey).Msg("Fetching issue")

	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,status,assignee", c.baseURL, issueKey)

	var issue Issue
	if err := c.doRequest("GET", endpoint, nil, &issue); err != nil {
		return nil, fmt.Errorf("failed to fetch issue %s: %w", issueKey, err)
	}

	return &issue, nil
}

// SearchIssues searches for issues by key or text
func (c *Client) SearchIssues(searchKey string) ([]Issue, error) {
	log.Debug().Str("search", searchKey).Msg("Searching issues")

	// Check if the search term looks like a Jira key (contains hyphen or is alphanumeric)
	var jql string
	if strings.Contains(searchKey, "-") || len(searchKey) < 3 {
		// Looks like a key, search by key
		jql = fmt.Sprintf("key = %s OR key ~ %s", searchKey, searchKey)
	} else {
		// Text search in summary and description
		jql = fmt.Sprintf("text ~ \"%s*\" OR summary ~ \"%s*\"", searchKey, searchKey)
	}

	if c.projectKey != "" {
		jql = fmt.Sprintf("(%s) AND project = %s", jql, c.projectKey)
	}
	jql = fmt.Sprintf("%s ORDER BY updated DESC", jql)

	// Use POST method with JSON body as recommended by Jira API v3
	endpoint := fmt.Sprintf("%s/rest/api/3/search/jql", c.baseURL)

	payload := map[string]interface{}{
		"jql":        jql,
		"fields":     []string{"summary", "status", "assignee"},
		"maxResults": 20,
	}

	var result SearchResult
	if err := c.doRequest("POST", endpoint, payload, &result); err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	log.Debug().Int("count", len(result.Issues)).Msg("Retrieved search results")
	return result.Issues, nil
}

// AddWorklog adds a worklog entry to an issue
func (c *Client) AddWorklog(issueKey string, timeSpentSeconds int, started time.Time, comment string) (*Worklog, error) {
	log.Debug().
		Str("issue", issueKey).
		Int("seconds", timeSpentSeconds).
		Msg("Adding worklog")

	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s/worklog", c.baseURL, issueKey)

	// Format started time in Jira format
	startedStr := started.Format("2006-01-02T15:04:05.000-0700")

	payload := map[string]interface{}{
		"timeSpentSeconds": timeSpentSeconds,
		"started":          startedStr,
	}

	if comment != "" {
		payload["comment"] = map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": comment,
						},
					},
				},
			},
		}
	}

	var worklog Worklog
	if err := c.doRequest("POST", endpoint, payload, &worklog); err != nil {
		return nil, fmt.Errorf("failed to add worklog: %w", err)
	}

	log.Info().
		Str("issue", issueKey).
		Str("time", formatSeconds(timeSpentSeconds)).
		Msg("Worklog added successfully")

	return &worklog, nil
}

// GetTodayWorklogs retrieves today's worklogs for the current user
func (c *Client) GetTodayWorklogs() ([]Worklog, error) {
	log.Debug().Msg("Fetching today's worklogs")

	// Get issues updated recently - JQL worklogDate filter may not be reliable
	jql := "assignee = currentUser() AND updated >= -7d"
	if c.projectKey != "" {
		jql = fmt.Sprintf("%s AND project = %s", jql, c.projectKey)
	}
	jql = fmt.Sprintf("%s ORDER BY updated DESC", jql)

	log.Debug().Str("jql", jql).Msg("Using JQL query")

	endpoint := fmt.Sprintf("%s/rest/api/3/search/jql", c.baseURL)
	payload := map[string]interface{}{
		"jql":        jql,
		"fields":     []string{"worklog", "summary", "key"},
		"maxResults": 100,
	}

	var result SearchResult
	if err := c.doRequest("POST", endpoint, payload, &result); err != nil {
		return nil, fmt.Errorf("failed to fetch today's issues: %w", err)
	}

	log.Debug().
		Int("total_issues", result.Total).
		Int("returned_issues", len(result.Issues)).
		Msg("Search result")

	// Get current user to filter worklogs
	currentUser, err := c.GetCurrentUser()
	if err != nil {
		log.Warn().Err(err).Msg("Could not fetch current user, will include all worklogs")
	}

	// Extract worklogs from issues
	worklogs := []Worklog{}
	today := time.Now().Format("2006-01-02")

	for _, issue := range result.Issues {
		log.Debug().
			Str("issue_key", issue.Key).
			Bool("has_worklog", issue.Fields.Worklog != nil).
			Msg("Processing issue")

		if issue.Fields.Worklog == nil {
			continue
		}

		log.Debug().
			Str("issue_key", issue.Key).
			Int("worklog_count", len(issue.Fields.Worklog.Worklogs)).
			Msg("Issue has worklogs")

		// Filter worklogs to only include today's entries by current user
		for _, wl := range issue.Fields.Worklog.Worklogs {
			// Check if this worklog is from today
			isToday := strings.HasPrefix(wl.Started, today)

			// Check if this worklog is by current user
			isByCurrentUser := true
			if currentUser != nil && wl.Author != nil {
				isByCurrentUser = wl.Author.AccountID == currentUser.AccountID
			}

			log.Debug().
				Str("issue_key", issue.Key).
				Str("started", wl.Started).
				Str("today", today).
				Bool("is_today", isToday).
				Bool("is_by_current_user", isByCurrentUser).
				Msg("Checking worklog")

			if isToday && isByCurrentUser {
				// Add issue context to worklog
				wl.IssueID = issue.ID
				worklogs = append(worklogs, wl)
			}
		}
	}

	log.Debug().Int("count", len(worklogs)).Msg("Retrieved today's worklogs")
	return worklogs, nil
}

// GetCurrentUser retrieves the current user's account information
func (c *Client) GetCurrentUser() (*IssueUser, error) {
	log.Debug().Msg("Fetching current user information")

	endpoint := fmt.Sprintf("%s/rest/api/3/myself", c.baseURL)

	var user IssueUser
	if err := c.doRequest("GET", endpoint, nil, &user); err != nil {
		return nil, fmt.Errorf("failed to fetch current user: %w", err)
	}

	log.Debug().
		Str("account_id", user.AccountID).
		Str("display_name", user.DisplayName).
		Msg("Retrieved current user")

	return &user, nil
}

// doRequest performs an HTTP request to the Jira API
func (c *Client) doRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)
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
			Msg("API request failed")
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
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
