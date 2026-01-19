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
		IssueKey:         "PROJ-123",
		TimeSpentSeconds: 7200,
		StartDate:        "2024-11-11",
		StartTime:        "10:00:00",
		Description:      "Test work",
		CreatedAt:        "2024-11-11T10:00:00Z",
	}

	if resp.TempoWorklogID != 12345 {
		t.Error("tempo worklog ID not set correctly")
	}

	if resp.JiraWorklogID != 67890 {
		t.Error("jira worklog ID not set correctly")
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
	tests := []struct {
		name           string
		startDate      string
		startTime      string
		expectedHour   int
		expectedMin    int
		timezoneName   string
		timezoneOffset int // hours offset from UTC
	}{
		{
			name:           "UTC time stays the same",
			startDate:      "2024-01-15",
			startTime:      "09:00:00",
			expectedHour:   9,
			expectedMin:    0,
			timezoneName:   "UTC",
			timezoneOffset: 0,
		},
		{
			name:           "UTC+3 (KSA) converts correctly",
			startDate:      "2024-01-15",
			startTime:      "09:00:00",
			expectedHour:   12,
			expectedMin:    0,
			timezoneName:   "Asia/Riyadh",
			timezoneOffset: 3,
		},
		{
			name:           "UTC-5 (EST) converts correctly",
			startDate:      "2024-01-15",
			startTime:      "14:00:00",
			expectedHour:   9,
			expectedMin:    0,
			timezoneName:   "America/New_York",
			timezoneOffset: -5,
		},
		{
			name:           "UTC+1 (CET) converts correctly",
			startDate:      "2024-01-15",
			startTime:      "08:00:00",
			expectedHour:   9,
			expectedMin:    0,
			timezoneName:   "Europe/Paris",
			timezoneOffset: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the local timezone for this test
			loc, err := time.LoadLocation(tt.timezoneName)
			if err != nil {
				t.Skipf("Could not load %s timezone: %v", tt.timezoneName, err)
			}

			// Temporarily set local timezone for the test
			originalLocal := time.Local
			time.Local = loc
			defer func() { time.Local = originalLocal }()

			// Create a WorklogResponse with UTC time
			wl := WorklogResponse{
				StartDate: tt.startDate,
				StartTime: tt.startTime,
			}

			// Get the local time string
			localTimeStr := wl.GetLocalStartTime()

			// Parse the result
			parsedTime, err := time.Parse("15:04:05", localTimeStr)
			if err != nil {
				t.Fatalf("Failed to parse returned time %q: %v", localTimeStr, err)
			}

			// Verify hour and minute
			if parsedTime.Hour() != tt.expectedHour {
				t.Errorf("Expected hour %d, got %d (timezone %s, offset UTC%+d)",
					tt.expectedHour, parsedTime.Hour(), tt.timezoneName, tt.timezoneOffset)
			}

			if parsedTime.Minute() != tt.expectedMin {
				t.Errorf("Expected minute %d, got %d", tt.expectedMin, parsedTime.Minute())
			}
		})
	}
}
