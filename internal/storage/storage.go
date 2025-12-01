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
	Label            string    `json:"label"`
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

// initSchema creates the database schema
func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS time_entries (
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

// AddTimeEntry adds a new time entry to the database
func (s *Storage) AddTimeEntry(entry *TimeEntry) error {
	log.Debug().
		Str("issue", entry.IssueKey).
		Int("seconds", entry.TimeSpentSeconds).
		Str("label", entry.Label).
		Msg("Adding time entry")

	query := `
		INSERT INTO time_entries (
			issue_key, issue_summary, time_spent_seconds, time_spent,
			label, comment, started, synced_to_jira, synced_to_tempo,
			jira_worklog_id, tempo_worklog_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(
		query,
		entry.IssueKey,
		entry.IssueSummary,
		entry.TimeSpentSeconds,
		entry.TimeSpent,
		entry.Label,
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
			label, comment, started, created_at, synced_to_jira, synced_to_tempo,
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
			&entry.Label,
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
			label, comment, started, created_at, synced_to_jira, synced_to_tempo,
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
			&entry.Label,
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
