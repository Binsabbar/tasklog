package storage

import (
	"testing"
	"time"
)

func TestNewStorage(t *testing.T) {
	// Use in-memory database for testing
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	if store.db == nil {
		t.Error("database connection is nil")
	}
}

func TestAddTimeEntry(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	entry := &TimeEntry{
		IssueKey:         "PROJ-123",
		IssueSummary:     "Test issue",
		TimeSpentSeconds: 3600,
		TimeSpent:        "1h",
		Comment:          "Test comment",
		Started:          time.Now(),
		SyncedToJira:     false,
		SyncedToTempo:    false,
	}

	err = store.AddTimeEntry(entry)
	if err != nil {
		t.Fatalf("failed to add time entry: %v", err)
	}

	if entry.ID == 0 {
		t.Error("expected ID to be set after insert")
	}
}

func TestUpdateTimeEntry(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	entry := &TimeEntry{
		IssueKey:         "PROJ-123",
		IssueSummary:     "Test issue",
		TimeSpentSeconds: 3600,
		TimeSpent:        "1h",
		Started:          time.Now(),
		SyncedToJira:     false,
		SyncedToTempo:    false,
	}

	err = store.AddTimeEntry(entry)
	if err != nil {
		t.Fatalf("failed to add time entry: %v", err)
	}

	// Update sync status
	entry.SyncedToJira = true
	jiraID := "12345"
	entry.JiraWorklogID = &jiraID
	entry.SyncedToTempo = true
	tempoID := "67890"
	entry.TempoWorklogID = &tempoID

	err = store.UpdateTimeEntry(entry)
	if err != nil {
		t.Fatalf("failed to update time entry: %v", err)
	}
}

func TestGetTodayEntries(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Add entries for today
	now := time.Now()
	for i := 0; i < 3; i++ {
		entry := &TimeEntry{
			IssueKey:         "PROJ-123",
			IssueSummary:     "Test issue",
			TimeSpentSeconds: 3600,
			TimeSpent:        "1h",
			Comment:          "Test work",
			Started:          now,
		}
		err = store.AddTimeEntry(entry)
		if err != nil {
			t.Fatalf("failed to add time entry: %v", err)
		}
	}

	// Add entry from yesterday
	yesterday := now.AddDate(0, 0, -1)
	entry := &TimeEntry{
		IssueKey:         "PROJ-456",
		IssueSummary:     "Yesterday issue",
		TimeSpentSeconds: 1800,
		TimeSpent:        "30m",
		Comment:          "Test work",
		Started:          yesterday,
	}
	err = store.AddTimeEntry(entry)
	if err != nil {
		t.Fatalf("failed to add yesterday entry: %v", err)
	}

	entries, err := store.GetTodayEntries()
	if err != nil {
		t.Fatalf("failed to get today entries: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries for today, got %d", len(entries))
	}
}

func TestGetUnsyncedEntries(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Add synced entry
	syncedEntry := &TimeEntry{
		IssueKey:         "PROJ-123",
		IssueSummary:     "Synced issue",
		TimeSpentSeconds: 3600,
		TimeSpent:        "1h",
		Comment:          "Test work",
		Started:          time.Now(),
		SyncedToJira:     true,
		SyncedToTempo:    true,
	}
	err = store.AddTimeEntry(syncedEntry)
	if err != nil {
		t.Fatalf("failed to add synced entry: %v", err)
	}

	// Add unsynced entries
	for i := 0; i < 2; i++ {
		entry := &TimeEntry{
			IssueKey:         "PROJ-456",
			IssueSummary:     "Unsynced issue",
			TimeSpentSeconds: 1800,
			TimeSpent:        "30m",
			Comment:          "Test work",
			Started:          time.Now(),
			SyncedToJira:     false,
			SyncedToTempo:    false,
		}
		err = store.AddTimeEntry(entry)
		if err != nil {
			t.Fatalf("failed to add unsynced entry: %v", err)
		}
	}

	// Add partially synced entry
	partialEntry := &TimeEntry{
		IssueKey:         "PROJ-789",
		IssueSummary:     "Partial sync",
		TimeSpentSeconds: 900,
		TimeSpent:        "15m",
		Comment:          "Test work",
		Started:          time.Now(),
		SyncedToJira:     true,
		SyncedToTempo:    false,
	}
	err = store.AddTimeEntry(partialEntry)
	if err != nil {
		t.Fatalf("failed to add partial entry: %v", err)
	}

	entries, err := store.GetUnsyncedEntries()
	if err != nil {
		t.Fatalf("failed to get unsynced entries: %v", err)
	}

	// Should return 3 entries: 2 fully unsynced + 1 partially synced
	if len(entries) != 3 {
		t.Errorf("expected 3 unsynced entries, got %d", len(entries))
	}
}

func TestGetTodayTotalSeconds(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Add entries with different durations
	now := time.Now()
	durations := []int{3600, 1800, 900} // 1h, 30m, 15m
	expectedTotal := 6300               // 1h 45m

	for _, duration := range durations {
		entry := &TimeEntry{
			IssueKey:         "PROJ-123",
			IssueSummary:     "Test issue",
			TimeSpentSeconds: duration,
			TimeSpent:        "test",
			Comment:          "Test work",
			Started:          now,
		}
		err = store.AddTimeEntry(entry)
		if err != nil {
			t.Fatalf("failed to add time entry: %v", err)
		}
	}

	total, err := store.GetTodayTotalSeconds()
	if err != nil {
		t.Fatalf("failed to get today total: %v", err)
	}

	if total != expectedTotal {
		t.Errorf("expected total %d seconds, got %d", expectedTotal, total)
	}
}

func TestGetTodayTotalSeconds_NoEntries(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	total, err := store.GetTodayTotalSeconds()
	if err != nil {
		t.Fatalf("failed to get today total: %v", err)
	}

	if total != 0 {
		t.Errorf("expected total 0 seconds for empty database, got %d", total)
	}
}

func TestClose(t *testing.T) {
	store, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("failed to close storage: %v", err)
	}
}
