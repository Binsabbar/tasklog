package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// Storage represents the SQLite storage layer
type Storage struct {
	db *sql.DB
}

// TimeEntry represents a time entry in the local cache
type TimeEntry struct {
	ID               int64     `json:"id"`
	IssueKey         string    `json:"issue_key"`
	IssueSummary     string    `json:"issue_summary"`
	TimeSpentSeconds int       `json:"time_spent_seconds"`
	TimeSpent        string    `json:"time_spent"`
	Comment          string    `json:"comment"`
	Started          time.Time `json:"started"`
	CreatedAt        time.Time `json:"created_at"`
	SyncedToJira     bool      `json:"synced_to_jira"`
	SyncedToTempo    bool      `json:"synced_to_tempo"`
	JiraWorklogID    *string   `json:"jira_worklog_id"`
	TempoWorklogID   *string   `json:"tempo_worklog_id"`
}

// NewStorage creates a new storage instance
func NewStorage(dbPath string) (*Storage, error) {
	log.Debug().Str("path", dbPath).Msg("Opening database")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}

	if err := storage.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Debug().Msg("Database initialized successfully")
	return storage, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// initSchema creates the database schema and runs migrations
func (s *Storage) initSchema() error {
	// Create schema_version table if it doesn't exist
	versionSchema := `
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY
	);
	`
	if _, err := s.db.Exec(versionSchema); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current schema version
	currentVersion, err := s.getSchemaVersion()
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	log.Debug().Int("version", currentVersion).Msg("Current schema version")

	// Run migrations
	if err := s.runMigrations(currentVersion); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create initial schema (for new installations)
	schema := `
	CREATE TABLE IF NOT EXISTS time_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_key TEXT NOT NULL,
		issue_summary TEXT NOT NULL,
		time_spent_seconds INTEGER NOT NULL,
		time_spent TEXT NOT NULL,
		comment TEXT NOT NULL,
		started DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		synced_to_jira BOOLEAN NOT NULL DEFAULT 0,
		synced_to_tempo BOOLEAN NOT NULL DEFAULT 0,
		jira_worklog_id TEXT,
		tempo_worklog_id TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_time_entries_issue_key ON time_entries(issue_key);
	CREATE INDEX IF NOT EXISTS idx_time_entries_started ON time_entries(started);
	CREATE INDEX IF NOT EXISTS idx_time_entries_created_at ON time_entries(created_at);
	CREATE INDEX IF NOT EXISTS idx_time_entries_synced ON time_entries(synced_to_jira, synced_to_tempo);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// getSchemaVersion returns the current schema version
func (s *Storage) getSchemaVersion() (int, error) {
	var version int
	err := s.db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		// No version set, check if time_entries table exists
		var tableName string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='time_entries'").Scan(&tableName)
		if err == sql.ErrNoRows {
			// Fresh installation
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		// Table exists but no version - this is v1
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}

// setSchemaVersion sets the schema version
func (s *Storage) setSchemaVersion(version int) error {
	_, err := s.db.Exec("DELETE FROM schema_version")
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
	return err
}

// runMigrations runs all migrations from the current version to the latest
func (s *Storage) runMigrations(currentVersion int) error {
	migrations := []struct {
		version int
		migrate func() error
	}{
		{2, s.migrateV1ToV2}, // Remove label column, make comment NOT NULL
	}

	for _, migration := range migrations {
		if currentVersion < migration.version {
			log.Info().Int("version", migration.version).Msg("Running migration")
			if err := migration.migrate(); err != nil {
				return fmt.Errorf("failed to migrate to version %d: %w", migration.version, err)
			}
			if err := s.setSchemaVersion(migration.version); err != nil {
				return fmt.Errorf("failed to set schema version: %w", err)
			}
			log.Info().Int("version", migration.version).Msg("Migration completed successfully")
		}
	}

	return nil
}

// migrateV1ToV2 removes the label column and makes comment NOT NULL
func (s *Storage) migrateV1ToV2() error {
	log.Info().Msg("Migrating database from v1 to v2: removing label column, making comment NOT NULL")

	// Check if label column exists
	var hasLabel bool
	rows, err := s.db.Query("PRAGMA table_info(time_entries)")
	if err != nil {
		return fmt.Errorf("failed to query table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		if name == "label" {
			hasLabel = true
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating columns: %w", err)
	}

	if !hasLabel {
		log.Debug().Msg("Label column does not exist, skipping migration")
		return nil
	}

	// SQLite doesn't support DROP COLUMN directly in older versions
	// We need to create a new table and copy data
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback is safe to call even after Commit
	}()

	// First, set default value for empty comments
	_, err = tx.Exec("UPDATE time_entries SET comment = label WHERE comment IS NULL OR comment = ''")
	if err != nil {
		return fmt.Errorf("failed to update empty comments: %w", err)
	}

	// Create new table with updated schema
	_, err = tx.Exec(`
		CREATE TABLE time_entries_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			issue_key TEXT NOT NULL,
			issue_summary TEXT NOT NULL,
			time_spent_seconds INTEGER NOT NULL,
			time_spent TEXT NOT NULL,
			comment TEXT NOT NULL,
			started DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			synced_to_jira BOOLEAN NOT NULL DEFAULT 0,
			synced_to_tempo BOOLEAN NOT NULL DEFAULT 0,
			jira_worklog_id TEXT,
			tempo_worklog_id TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new table: %w", err)
	}

	// Copy data from old table to new table
	_, err = tx.Exec(`
		INSERT INTO time_entries_new (
			id, issue_key, issue_summary, time_spent_seconds, time_spent,
			comment, started, created_at, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		)
		SELECT 
			id, issue_key, issue_summary, time_spent_seconds, time_spent,
			comment, started, created_at, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		FROM time_entries
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Drop old table
	_, err = tx.Exec("DROP TABLE time_entries")
	if err != nil {
		return fmt.Errorf("failed to drop old table: %w", err)
	}

	// Rename new table
	_, err = tx.Exec("ALTER TABLE time_entries_new RENAME TO time_entries")
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}

	// Recreate indexes
	_, err = tx.Exec(`
		CREATE INDEX idx_time_entries_issue_key ON time_entries(issue_key);
		CREATE INDEX idx_time_entries_started ON time_entries(started);
		CREATE INDEX idx_time_entries_created_at ON time_entries(created_at);
		CREATE INDEX idx_time_entries_synced ON time_entries(synced_to_jira, synced_to_tempo);
	`)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().Msg("Successfully migrated database: label column removed, comment is now NOT NULL")
	return nil
}

// AddTimeEntry adds a new time entry to the database
func (s *Storage) AddTimeEntry(entry *TimeEntry) error {
	log.Debug().
		Str("issue", entry.IssueKey).
		Int("seconds", entry.TimeSpentSeconds).
		Msg("Adding time entry")

	query := `
		INSERT INTO time_entries (
			issue_key, issue_summary, time_spent_seconds, time_spent,
			comment, started, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(
		query,
		entry.IssueKey,
		entry.IssueSummary,
		entry.TimeSpentSeconds,
		entry.TimeSpent,
		entry.Comment,
		entry.Started,
		entry.SyncedToJira,
		entry.SyncedToTempo,
		entry.JiraWorklogID,
		entry.TempoWorklogID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert time entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted ID: %w", err)
	}

	entry.ID = id
	log.Info().Int64("id", id).Msg("Time entry added to local cache")
	return nil
}

// UpdateTimeEntry updates an existing time entry
func (s *Storage) UpdateTimeEntry(entry *TimeEntry) error {
	log.Debug().Int64("id", entry.ID).Msg("Updating time entry")

	query := `
		UPDATE time_entries SET
			synced_to_jira = ?,
			synced_to_tempo = ?,
			jira_worklog_id = ?,
			tempo_worklog_id = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(
		query,
		entry.SyncedToJira,
		entry.SyncedToTempo,
		entry.JiraWorklogID,
		entry.TempoWorklogID,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update time entry: %w", err)
	}

	log.Debug().Int64("id", entry.ID).Msg("Time entry updated")
	return nil
}

// GetTodayEntries retrieves all time entries for today
func (s *Storage) GetTodayEntries() ([]TimeEntry, error) {
	log.Debug().Msg("Fetching today's entries")

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	query := `
		SELECT 
			id, issue_key, issue_summary, time_spent_seconds, time_spent,
			comment, started, created_at, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		FROM time_entries
		WHERE started >= ? AND started < ?
		ORDER BY started DESC
	`

	rows, err := s.db.Query(query, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("failed to query time entries: %w", err)
	}
	defer rows.Close()

	var entries []TimeEntry
	for rows.Next() {
		var entry TimeEntry
		err := rows.Scan(
			&entry.ID,
			&entry.IssueKey,
			&entry.IssueSummary,
			&entry.TimeSpentSeconds,
			&entry.TimeSpent,
			&entry.Comment,
			&entry.Started,
			&entry.CreatedAt,
			&entry.SyncedToJira,
			&entry.SyncedToTempo,
			&entry.JiraWorklogID,
			&entry.TempoWorklogID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating time entries: %w", err)
	}

	log.Debug().Int("count", len(entries)).Msg("Retrieved today's entries")
	return entries, nil
}

// GetUnsyncedEntries retrieves entries that haven't been synced to Jira or Tempo
func (s *Storage) GetUnsyncedEntries() ([]TimeEntry, error) {
	log.Debug().Msg("Fetching unsynced entries")

	query := `
		SELECT 
			id, issue_key, issue_summary, time_spent_seconds, time_spent,
			comment, started, created_at, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		FROM time_entries
		WHERE synced_to_jira = 0 OR synced_to_tempo = 0
		ORDER BY started ASC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unsynced entries: %w", err)
	}
	defer rows.Close()

	var entries []TimeEntry
	for rows.Next() {
		var entry TimeEntry
		err := rows.Scan(
			&entry.ID,
			&entry.IssueKey,
			&entry.IssueSummary,
			&entry.TimeSpentSeconds,
			&entry.TimeSpent,
			&entry.Comment,
			&entry.Started,
			&entry.CreatedAt,
			&entry.SyncedToJira,
			&entry.SyncedToTempo,
			&entry.JiraWorklogID,
			&entry.TempoWorklogID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating unsynced entries: %w", err)
	}

	log.Debug().Int("count", len(entries)).Msg("Retrieved unsynced entries")
	return entries, nil
}

// GetTodayTotalSeconds calculates total seconds logged today
func (s *Storage) GetTodayTotalSeconds() (int, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	var total sql.NullInt64
	query := `
		SELECT SUM(time_spent_seconds)
		FROM time_entries
		WHERE started >= ? AND started < ?
	`

	err := s.db.QueryRow(query, startOfDay, endOfDay).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate total: %w", err)
	}

	if !total.Valid {
		return 0, nil
	}

	return int(total.Int64), nil
}
