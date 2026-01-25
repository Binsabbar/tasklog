package tempo

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("tempo-token-123")

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.apiToken != "tempo-token-123" {
		t.Error("expected apiToken to be set correctly")
	}

	if client.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout to be 30s, got %v", client.httpClient.Timeout)
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

func TestWorklogRequestStructure(t *testing.T) {
	req := WorklogRequest{
		IssueID:          "12345",
		AuthorAccountID:  "account-123",
		TimeSpentSeconds: 7200,
		StartDate:        "2024-11-11",
		StartTime:        "10:00:00",
		Description:      "Test work",
	}

	if req.IssueID != "12345" {
		t.Error("issue ID not set correctly")
	}

	if req.TimeSpentSeconds != 7200 {
		t.Error("time spent seconds not set correctly")
	}
}

func TestWorklogResponseStructure(t *testing.T) {
	resp := WorklogResponse{
		TempoWorklogID:   12345,
		JiraWorklogID:    67890,
		TimeSpentSeconds: 7200,
		StartDate:        "2024-11-11",
		StartTime:        "10:00:00",
		Description:      "Test work",
		CreatedAt:        "2024-11-11T10:00:00Z",
	}
	resp.Issue.Key = "PROJ-123"
	resp.Issue.ID = 99999

	if resp.TempoWorklogID != 12345 {
		t.Error("tempo worklog ID not set correctly")
	}

	if resp.JiraWorklogID != 67890 {
		t.Error("jira worklog ID not set correctly")
	}

	if resp.IssueKey() != "PROJ-123" {
		t.Errorf("expected issue key PROJ-123, got %s", resp.IssueKey())
	}
}

func TestWorklogAttributeStructure(t *testing.T) {
	attr := WorklogAttribute{
		Key:   "label",
		Value: "development",
	}

	if attr.Key != "label" {
		t.Error("attribute key not set correctly")
	}

	if attr.Value != "development" {
		t.Error("attribute value not set correctly")
	}
}

func TestGetLocalStartTime(t *testing.T) {
	t.Run("uses UTC time when available", func(t *testing.T) {
		// Set local timezone to UTC for predictable test
		originalLocal := time.Local
		time.Local = time.UTC
		defer func() { time.Local = originalLocal }()

		wl := WorklogResponse{
			StartDate:        "2024-01-15",
			StartTime:        "12:00:00", // This should be ignored
			StartDateTimeUtc: "2024-01-15T09:00:00Z",
		}

		result := wl.GetLocalStartTime()
		expected := "09:00:00" // UTC time converted to local (which is UTC in this test)
		if result != expected {
			t.Errorf("GetLocalStartTime() = %q, want %q", result, expected)
		}
	})

	t.Run("falls back to startTime when UTC not available", func(t *testing.T) {
		wl := WorklogResponse{
			StartDate: "2024-01-15",
			StartTime: "14:30:45",
			// StartDateTimeUtc not set
		}

		result := wl.GetLocalStartTime()
		if result != "14:30:45" {
			t.Errorf("GetLocalStartTime() = %q, want %q", result, "14:30:45")
		}
	})
}
