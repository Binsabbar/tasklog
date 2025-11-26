package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"tasklog/internal/jira"
	"tasklog/internal/storage"
	"tasklog/internal/tempo"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show today's time tracking summary",
	Long:  `Displays a summary of all time entries logged today.`,
	RunE:  runSummary,
}

func init() {
	rootCmd.AddCommand(summaryCmd)
}

func runSummary(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := checkConfig()
	if err != nil {
		return err
	}

	// Tempo is required for summary
	if !cfg.Tempo.Enabled || cfg.Tempo.APIToken == "" {
		return fmt.Errorf("tempo must be enabled and configured to use summary command")
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

	return showTodaySummary(store, jiraClient, tempoClient, cfg)
}
