package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"tasklog/internal/jira"
	"tasklog/internal/storage"
	"tasklog/internal/tempo"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync unsynced time entries to Jira and Tempo",
	Long:  `Attempts to sync any time entries that failed to sync to Jira or Tempo.`,
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
	tempoClient := tempo.NewClient(cfg.Tempo.APIToken)

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
			}
		}

		// Sync to Tempo if not synced and Tempo is enabled
		if !entry.SyncedToTempo && cfg.Tempo.Enabled && cfg.Tempo.APIToken != "" {
			log.Debug().Int64("id", entry.ID).Msg("Syncing to Tempo")

			// Get issue details for issue ID
			issue, err := jiraClient.GetIssue(entry.IssueKey)
			if err != nil {
				log.Error().Err(err).Int64("id", entry.ID).Msg("Failed to get issue details")
				fmt.Printf("  ✗ Failed to get issue details: %v\n", err)
				failureCount++
			} else {
				// Get current user's account ID
				currentUser, err := jiraClient.GetCurrentUser()
				if err != nil {
					log.Error().Err(err).Int64("id", entry.ID).Msg("Failed to get current user")
					fmt.Printf("  ✗ Failed to get current user: %v\n", err)
					failureCount++
				} else {
					tempoWorklog, err := tempoClient.AddWorklog(issue.ID, currentUser.AccountID, entry.TimeSpentSeconds, entry.Started, entry.Label, entry.Comment)
					if err != nil {
						log.Error().Err(err).Int64("id", entry.ID).Msg("Failed to sync to Tempo")
						fmt.Printf("  ✗ Failed to sync to Tempo: %v\n", err)
						failureCount++
					} else {
						entry.SyncedToTempo = true
						tempoID := fmt.Sprintf("%d", tempoWorklog.TempoWorklogID); entry.TempoWorklogID = &tempoID
						fmt.Println("  ✓ Synced to Tempo")
					}
				}
			}
		} else if !cfg.Tempo.Enabled {
			// Mark as synced if Tempo is not enabled
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
