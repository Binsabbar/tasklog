package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://example.atlassian.net", "user@example.com", "token123", "PROJ")

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.baseURL != "https://example.atlassian.net" {
		t.Errorf("expected baseURL to be set correctly, got %s", client.baseURL)
	}

	if client.username != "user@example.com" {
		t.Errorf("expected username to be set correctly")
	}

	if client.apiToken != "token123" {
		t.Errorf("expected apiToken to be set correctly")
	}

	if client.projectKey != "PROJ" {
		t.Errorf("expected projectKey to be set correctly, got %s", client.projectKey)
	}

	if client.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClient_TrimTrailingSlash(t *testing.T) {
	client := NewClient("https://example.atlassian.net/", "user@example.com", "token123", "PROJ")

	if client.baseURL != "https://example.atlassian.net" {
		t.Errorf("expected trailing slash to be trimmed, got %s", client.baseURL)
	}
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{3600, "1h"},
		{1800, "30m"},
		{5400, "1h 30m"},
		{7200, "2h"},
		{300, "5m"},
		{0, "0m"},
		{7260, "2h 1m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSeconds(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatSeconds(%d) = %s, want %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestIssueStructures(t *testing.T) {
	// Test that the structures can be created and marshaled
	issue := Issue{
		Key: "PROJ-123",
		Fields: IssueFields{
			Summary: "Test issue",
			Status: IssueStatus{
				Name: "In Progress",
			},
			Assignee: &IssueUser{
				DisplayName:  "John Doe",
				EmailAddress: "john@example.com",
			},
		},
	}

	if issue.Key != "PROJ-123" {
		t.Error("issue key not set correctly")
	}

	if issue.Fields.Summary != "Test issue" {
		t.Error("issue summary not set correctly")
	}

	if issue.Fields.Status.Name != "In Progress" {
		t.Error("issue status not set correctly")
	}

	if issue.Fields.Assignee == nil {
		t.Error("assignee should not be nil")
	}
}

func TestWorklogStructure(t *testing.T) {
	worklog := Worklog{
		ID:               "12345",
		TimeSpent:        "2h",
		TimeSpentSeconds: 7200,
		Started:          "2024-11-11T10:00:00.000+0000",
		Comment:          []byte(`"Test comment"`),
	}

	if worklog.ID != "12345" {
		t.Error("worklog ID not set correctly")
	}

	if worklog.TimeSpentSeconds != 7200 {
		t.Error("time spent seconds not set correctly")
	}
}

func TestGetInProgressIssues_DefaultStatus(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		// Parse request body to verify JQL
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		jql, ok := payload["jql"].(string)
		if !ok {
			t.Fatal("jql not found in request")
		}

		// Verify default status is "In Progress"
		expectedJQL := "assignee = currentUser() AND status = 'In Progress' AND project = TEST ORDER BY updated DESC"
		if jql != expectedJQL {
			t.Errorf("expected JQL:\n%s\ngot:\n%s", expectedJQL, jql)
		}

		// Return mock response
		response := SearchResult{
			Issues: []Issue{
				{
					Key: "TEST-123",
					Fields: IssueFields{
						Summary: "Test issue",
						Status:  IssueStatus{Name: "In Progress"},
					},
				},
			},
			Total: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@example.com", "token", "TEST")
	issues, err := client.GetInProgressIssues(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].Key != "TEST-123" {
		t.Errorf("expected issue key TEST-123, got %s", issues[0].Key)
	}
}

func TestGetInProgressIssues_MultipleStatuses(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to verify JQL
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		jql, ok := payload["jql"].(string)
		if !ok {
			t.Fatal("jql not found in request")
		}

		// Verify multiple statuses are included
		expectedJQL := "assignee = currentUser() AND status IN ('In Progress', 'In Review', 'Testing') AND project = TEST ORDER BY updated DESC"
		if jql != expectedJQL {
			t.Errorf("expected JQL:\n%s\ngot:\n%s", expectedJQL, jql)
		}

		// Return mock response
		response := SearchResult{
			Issues: []Issue{
				{
					Key: "TEST-123",
					Fields: IssueFields{
						Summary: "In Progress issue",
						Status:  IssueStatus{Name: "In Progress"},
					},
				},
				{
					Key: "TEST-456",
					Fields: IssueFields{
						Summary: "In Review issue",
						Status:  IssueStatus{Name: "In Review"},
					},
				},
			},
			Total: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@example.com", "token", "TEST")
	statuses := []string{"In Progress", "In Review", "Testing"}
	issues, err := client.GetInProgressIssues(statuses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}
}

func TestGetInProgressIssues_SingleCustomStatus(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to verify JQL
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		jql, ok := payload["jql"].(string)
		if !ok {
			t.Fatal("jql not found in request")
		}

		// Verify single custom status
		expectedJQL := "assignee = currentUser() AND status = 'In Review' AND project = TEST ORDER BY updated DESC"
		if jql != expectedJQL {
			t.Errorf("expected JQL:\n%s\ngot:\n%s", expectedJQL, jql)
		}

		// Return mock response
		response := SearchResult{
			Issues: []Issue{
				{
					Key: "TEST-789",
					Fields: IssueFields{
						Summary: "Review issue",
						Status:  IssueStatus{Name: "In Review"},
					},
				},
			},
			Total: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@example.com", "token", "TEST")
	statuses := []string{"In Review"}
	issues, err := client.GetInProgressIssues(statuses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].Key != "TEST-789" {
		t.Errorf("expected issue key TEST-789, got %s", issues[0].Key)
	}
}
