package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"tasklog/internal/jira"
	"tasklog/internal/storage"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync unsynced time entries to Jira and Tempo",
	Long:  `Attempts to sync any time entries that failed to sync to Jira or Tempo.` + configHelp,
	RunE:  runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := checkConfig()
	if err != nil {
		return err
	}

	// Initialize clients
	jiraClient := jira.NewClient(cfg.Jira.URL, cfg.Jira.Username, cfg.Jira.APIToken, cfg.Jira.ProjectKey)

	// Initialize storage
	store, err := storage.NewStorage(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Get unsynced entries
	entries, err := store.GetUnsyncedEntries()
	if err != nil {
		return fmt.Errorf("failed to fetch unsynced entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("✓ All entries are synced")
		return nil
	}

	fmt.Printf("Found %d unsynced entries\n\n", len(entries))

	successCount := 0
	failureCount := 0

	for i, entry := range entries {
		fmt.Printf("[%d/%d] Syncing %s - %s\n", i+1, len(entries), entry.IssueKey, entry.TimeSpent)

		// Sync to Jira if not synced
		if !entry.SyncedToJira {
			log.Debug().Int64("id", entry.ID).Msg("Syncing to Jira")
			worklog, err := jiraClient.AddWorklog(entry.IssueKey, entry.TimeSpentSeconds, entry.Started, entry.Comment)
			if err != nil {
				log.Error().Err(err).Int64("id", entry.ID).Msg("Failed to sync to Jira")
				fmt.Printf("  ✗ Failed to sync to Jira: %v\n", err)
				failureCount++
			} else {
				entry.SyncedToJira = true
				entry.JiraWorklogID = &worklog.ID
				fmt.Println("  ✓ Synced to Jira")

				// If Tempo is enabled, Jira automatically creates a Tempo worklog
				if cfg.Tempo.Enabled {
					entry.SyncedToTempo = true
					fmt.Println("  ✓ Tempo worklog created automatically by Jira")
				}
			}
		}

		// Mark as synced if Tempo is not enabled
		if !cfg.Tempo.Enabled && !entry.SyncedToTempo {
			entry.SyncedToTempo = true
		}

		// Update storage
		if err := store.UpdateTimeEntry(&entry); err != nil {
			log.Error().Err(err).Int64("id", entry.ID).Msg("Failed to update entry")
		}

		if entry.SyncedToJira && entry.SyncedToTempo {
			successCount++
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Sync complete: %d successful, %d failed\n", successCount, failureCount)

	return nil
}
