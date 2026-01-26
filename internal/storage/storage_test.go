package storage

import (
	"os"
	"path/filepath"
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

// TestNewStorage_WithTildePath tests database creation with tilde in path
func TestNewStorage_WithTildePath(t *testing.T) {
	// Save original HOME environment variable
	originalHome := os.Getenv("HOME")

	// Create a temporary directory to use as fake home
	tmpHome := t.TempDir()

	// Set HOME to temporary directory for this test
	// This is safe because:
	// 1. t.Setenv automatically restores the original value after the test
	// 2. Each test runs in isolation
	// 3. No security risk - we're just testing path expansion logic
	t.Setenv("HOME", tmpHome)

	// Verify HOME was set correctly
	if os.Getenv("HOME") != tmpHome {
		t.Fatal("failed to set HOME environment variable")
	}

	// Use tilde path - should expand to tmpHome
	dbPath := "~/.tasklog/test.db"
	expectedPath := filepath.Join(tmpHome, ".tasklog", "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage with tilde path: %v", err)
	}
	defer store.Close()

	// Verify database file was created at the expanded path
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("database file was not created at expected path: %s", expectedPath)
	}

	// Verify HOME is restored after test (t.Setenv handles this automatically)
	// This is just for demonstration - the cleanup happens in t.Cleanup()
	t.Cleanup(func() {
		if os.Getenv("HOME") != originalHome {
			t.Logf("Note: HOME will be restored to %s after test", originalHome)
		}
	})
}

// TestNewStorage_WithNestedPath tests database creation in nested non-existent directory
func TestNewStorage_WithNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "level1", "level2", "level3", "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage with nested path: %v", err)
	}
	defer store.Close()

	// Verify all directories were created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("parent directories were not created")
	}

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

// TestNewStorage_WithExistingDirectory tests that existing directories work correctly
func TestNewStorage_WithExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create the directory
	dbDir := filepath.Join(tmpDir, "existing")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	dbPath := filepath.Join(dbDir, "test.db")
	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage in existing directory: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
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

func TestMigrationV1ToV2(t *testing.T) {
	// Create a temporary database file for migration testing
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_migration.db"

	// Create storage with old schema (v1)
	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create old schema manually with label column
	_, err = store.db.Exec(`
		DROP TABLE IF EXISTS time_entries;
		DROP TABLE IF EXISTS schema_version;
	`)
	if err != nil {
		t.Fatalf("failed to drop tables: %v", err)
	}

	// Create old schema with label column
	_, err = store.db.Exec(`
		CREATE TABLE time_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			issue_key TEXT NOT NULL,
			issue_summary TEXT NOT NULL,
			time_spent_seconds INTEGER NOT NULL,
			time_spent TEXT NOT NULL,
			label TEXT NOT NULL,
			comment TEXT,
			started DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			synced_to_jira BOOLEAN NOT NULL DEFAULT 0,
			synced_to_tempo BOOLEAN NOT NULL DEFAULT 0,
			jira_worklog_id TEXT,
			tempo_worklog_id TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create old schema: %v", err)
	}

	// Insert test data with label and empty comment
	now := time.Now()
	_, err = store.db.Exec(`
		INSERT INTO time_entries (
			issue_key, issue_summary, time_spent_seconds, time_spent,
			label, comment, started
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "PROJ-123", "Test issue", 3600, "1h", "development", "", now)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Insert test data with label and comment
	_, err = store.db.Exec(`
		INSERT INTO time_entries (
			issue_key, issue_summary, time_spent_seconds, time_spent,
			label, comment, started
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "PROJ-456", "Test issue 2", 1800, "30m", "testing", "Custom comment", now)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Create schema_version table with version 1
	_, err = store.db.Exec(`
		CREATE TABLE schema_version (version INTEGER PRIMARY KEY);
		INSERT INTO schema_version (version) VALUES (1);
	`)
	if err != nil {
		t.Fatalf("failed to create schema_version: %v", err)
	}

	store.Close()

	// Reopen storage to trigger migration
	store, err = NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}
	defer store.Close()

	// Verify schema version is now 2
	version, err := store.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 2 {
		t.Errorf("expected schema version 2, got %d", version)
	}

	// Verify label column is removed
	rows, err := store.db.Query("PRAGMA table_info(time_entries)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}
	defer rows.Close()

	hasLabel := false
	hasComment := false
	commentNotNull := false

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		if name == "label" {
			hasLabel = true
		}
		if name == "comment" {
			hasComment = true
			commentNotNull = notnull == 1
		}
	}

	if hasLabel {
		t.Error("label column should have been removed")
	}
	if !hasComment {
		t.Error("comment column should exist")
	}
	if !commentNotNull {
		t.Error("comment column should be NOT NULL")
	}

	// Verify data migration
	// Entry with empty comment should now have label value as comment
	var comment1 string
	err = store.db.QueryRow("SELECT comment FROM time_entries WHERE issue_key = 'PROJ-123'").Scan(&comment1)
	if err != nil {
		t.Fatalf("failed to query first entry: %v", err)
	}
	if comment1 != "development" {
		t.Errorf("expected comment to be 'development' (from label), got '%s'", comment1)
	}

	// Entry with comment should keep its comment
	var comment2 string
	err = store.db.QueryRow("SELECT comment FROM time_entries WHERE issue_key = 'PROJ-456'").Scan(&comment2)
	if err != nil {
		t.Fatalf("failed to query second entry: %v", err)
	}
	if comment2 != "Custom comment" {
		t.Errorf("expected comment to be 'Custom comment', got '%s'", comment2)
	}
}

func TestFreshInstallation(t *testing.T) {
	// Create storage with no existing database
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_fresh.db"

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Fresh installation should have schema version 2 after migration runs
	// (migration is idempotent and checks if label column exists)
	version, err := store.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}

	// For a fresh install, the migration runs but detects no label column
	// and sets version to 2
	if version != 2 {
		t.Errorf("expected schema version 2 for fresh install, got %d", version)
	}

	// Verify table structure is correct
	rows, err := store.db.Query("PRAGMA table_info(time_entries)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}
	defer rows.Close()

	hasLabel := false
	hasComment := false
	commentNotNull := false

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		if name == "label" {
			hasLabel = true
		}
		if name == "comment" {
			hasComment = true
			commentNotNull = notnull == 1
		}
	}

	if hasLabel {
		t.Error("fresh install should not have label column")
	}
	if !hasComment {
		t.Error("comment column should exist")
	}
	if !commentNotNull {
		t.Error("comment column should be NOT NULL")
	}

	// Test that we can add an entry with comment
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
}
