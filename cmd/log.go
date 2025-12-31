package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"tasklog/internal/config"
	"tasklog/internal/jira"
	"tasklog/internal/storage"
	"tasklog/internal/tempo"
	"tasklog/internal/timeparse"
	"tasklog/internal/ui"
)

var (
	shortcutName string
	taskKey      string
	timeSpent    string
	label        string
	startedAt    string
)

var logCmd = &cobra.Command{
	Use:   "log [shortcut-name]",
	Short: "Log time to a task",
	Long: `Interactively log time to a Jira task. 
You can use shortcuts, select from in-progress tasks, or search for tasks.

Examples:
  tasklog log              # Interactive mode
  tasklog log daily        # Use 'daily' shortcut
  tasklog log standup      # Use 'standup' shortcut
  tasklog log -t PROJ-123  # Log to specific task` + configHelp,
	Args: cobra.MaximumNArgs(1),
	RunE: runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)

	logCmd.Flags().StringVarP(&taskKey, "task", "t", "", "Task key (e.g., PROJ-123)")
	logCmd.Flags().StringVarP(&timeSpent, "time", "d", "", "Time spent (e.g., 2h 30m, 2.5h, 150m)")
	logCmd.Flags().StringVarP(&label, "label", "l", "", "Work log label")
	logCmd.Flags().StringVarP(&startedAt, "at", "a", "", "When work was performed (e.g., 2pm, yesterday, 2h ago)")

	// Set custom usage template to show available shortcuts
	logCmd.SetUsageFunc(logUsageFunc)
}

func logUsageFunc(cmd *cobra.Command) error {
	// Print usage
	fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s\n\n", cmd.UseLine())

	// Try to load config and show available shortcuts
	cfg, err := config.Load()
	if err == nil && len(cfg.Jira.Shortcuts) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "Available Shortcuts:\n")
		for _, sc := range cfg.Jira.Shortcuts {
			timeInfo := ""
			if sc.Time != "" {
				timeInfo = fmt.Sprintf(" (%s)", sc.Time)
			}
			fmt.Fprintf(cmd.OutOrStderr(), "  %-15s %s - %s%s\n", sc.Name, sc.Task, sc.Label, timeInfo)
		}
		fmt.Fprintf(cmd.OutOrStderr(), "\n")
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Flags:\n")
	fmt.Fprintf(cmd.OutOrStderr(), "%s", cmd.Flags().FlagUsages())

	return nil
}

func runLog(cmd *cobra.Command, args []string) error {
	// Check if first argument is a shortcut name
	if len(args) > 0 {
		shortcutName = args[0]
	}

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

	var selectedIssue *jira.Issue
	var timeSeconds int
	var selectedLabel string

	// Check if using a shortcut
	if shortcutName != "" {
		Out.Debug(fmt.Sprintf("Using shortcut: %s", shortcutName))

		shortcut, found := cfg.GetShortcut(shortcutName)
		if !found {
			return fmt.Errorf("shortcut '%s' not found in configuration", shortcutName)
		}

		// Use shortcut values
		if taskKey == "" {
			taskKey = shortcut.Task
		}
		if timeSpent == "" && shortcut.Time != "" {
			timeSpent = shortcut.Time
		}
		if label == "" {
			label = shortcut.Label
		}
	}

	// Get task
	if taskKey != "" {
		Out.Debug(fmt.Sprintf("Fetching specified task: %s", taskKey))
		issue, err := jiraClient.GetIssue(taskKey)
		if err != nil {
			return fmt.Errorf("failed to fetch task %s: %w", taskKey, err)
		}
		selectedIssue = issue
		Out.Printf("Task: %s - %s\n", selectedIssue.Key, selectedIssue.Fields.Summary)
	} else {
		// Interactive task selection
		Out.Debug("Fetching in-progress tasks")
		inProgressIssues, err := jiraClient.GetInProgressIssues(cfg.Jira.TaskStatuses)
		if err != nil {
			return fmt.Errorf("failed to fetch in-progress tasks: %w", err)
		}

		selectedIssue, err = ui.SelectTask(inProgressIssues)
		if err != nil {
			return fmt.Errorf("failed to select task: %w", err)
		}

		// If user chose to search, perform the search
		if selectedIssue.Fields.Summary == "" {
			searchResults, err := jiraClient.SearchIssues(selectedIssue.Key)
			if err != nil {
				return fmt.Errorf("failed to search tasks: %w", err)
			}

			selectedIssue, err = ui.SelectFromSearchResults(searchResults)
			if err != nil {
				return fmt.Errorf("failed to select from search results: %w", err)
			}

			// Fetch full issue details
			issue, err := jiraClient.GetIssue(selectedIssue.Key)
			if err != nil {
				return fmt.Errorf("failed to fetch task details: %w", err)
			}
			selectedIssue = issue
		}
	}

	// Get time spent
	if timeSpent != "" {
		timeSeconds, err = timeparse.Parse(timeSpent)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
	} else {
		timeStr, err := ui.PromptTimeSpent()
		if err != nil {
			return fmt.Errorf("failed to get time spent: %w", err)
		}

		timeSeconds, err = timeparse.Parse(timeStr)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
	}

	// Get label
	if label != "" {
		if !cfg.IsLabelAllowed(label) {
			return fmt.Errorf("label '%s' is not in the allowed labels list", label)
		}
		selectedLabel = label
	} else {
		selectedLabel, err = ui.SelectLabel(cfg.Labels.AllowedLabels)
		if err != nil {
			return fmt.Errorf("failed to select label: %w", err)
		}

		if !cfg.IsLabelAllowed(selectedLabel) {
			return fmt.Errorf("label '%s' is not allowed", selectedLabel)
		}
	}

	// Get optional comment
	comment, err := ui.PromptComment()
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	// Get start time
	var started time.Time
	if startedAt != "" {
		// Parse from --at flag
		var err error
		started, err = timeparse.ParseDateTime(startedAt)
		if err != nil {
			return fmt.Errorf("invalid time format for --at: %w", err)
		}
	} else if shortcutName == "" && taskKey == "" {
		// Interactive mode AND not using shortcuts: prompt for start time
		var err error
		started, err = ui.PromptStartTime()
		if err != nil {
			return fmt.Errorf("failed to get start time: %w", err)
		}
	} else {
		// Shortcut or CLI mode without --at: use current time
		started = time.Now()
	}

	// Confirm before logging
	Out.Printf("\n")
	Out.Printf("Task:    %s - %s\n", selectedIssue.Key, selectedIssue.Fields.Summary)
	Out.Printf("Time:    %s\n", timeparse.Format(timeSeconds))
	Out.Printf("Started: %s\n", started.Format("Mon Jan 2 15:04"))
	Out.Printf("Label:   %s\n", selectedLabel)
	if comment != "" {
		Out.Printf("Comment: %s\n", comment)
	}
	Out.Printf("\n")

	confirmed, err := ui.Confirm("Log this time entry?")
	if err != nil {
		return fmt.Errorf("failed to confirm: %w", err)
	}

	if !confirmed {
		Out.Println("Cancelled.")
		return nil
	}

	// Create time entry
	entry := &storage.TimeEntry{
		IssueKey:         selectedIssue.Key,
		IssueSummary:     selectedIssue.Fields.Summary,
		TimeSpentSeconds: timeSeconds,
		TimeSpent:        timeparse.Format(timeSeconds),
		Label:            selectedLabel,
		Comment:          comment,
		Started:          started,
		SyncedToJira:     false,
		SyncedToTempo:    false,
	}

	// Save to local storage first
	if err := store.AddTimeEntry(entry); err != nil {
		return fmt.Errorf("failed to save time entry locally: %w", err)
	}

	Out.Success("Saved to local cache")

	// Log to Jira
	Out.Debug("Logging to Jira")
	worklog, err := jiraClient.AddWorklog(selectedIssue.Key, timeSeconds, started, comment)
	if err != nil {
		Out.Error(fmt.Errorf("failed to log to Jira: %w", err))
		Out.Warn(fmt.Sprintf("Failed to log to Jira: %v", err))
	} else {
		entry.SyncedToJira = true
		entry.JiraWorklogID = &worklog.ID
		Out.Success("Logged to Jira")

		// If Tempo is enabled, Jira automatically creates a Tempo worklog
		// Mark as synced to Tempo since it's handled by Jira
		if cfg.Tempo.Enabled {
			entry.SyncedToTempo = true
			Out.Success("Tempo worklog created automatically by Jira")
		}
	}

	// Mark as synced if Tempo is not enabled
	if !cfg.Tempo.Enabled {
		entry.SyncedToTempo = true
	}

	// Update storage with sync status
	if err := store.UpdateTimeEntry(entry); err != nil {
		Out.Error(fmt.Errorf("failed to update time entry sync status: %w", err))
	}

	// Show today's summary
	Out.Println()
	if cfg.Tempo.Enabled && cfg.Tempo.APIToken != "" {
		if err := showTodaySummary(store, jiraClient, tempoClient, cfg); err != nil {
			Out.Error(fmt.Errorf("failed to show summary: %w", err))
		}
	} else {
		Out.Printf("═══════════════════════════════════════════\n")
		Out.Printf("📊 Summary is disabled\n")
		Out.Printf("═══════════════════════════════════════════\n")
		Out.Printf("To enable time tracking summary, configure Tempo API in your config:\n")
		Out.Printf("  tempo:\n")
		Out.Printf("    enabled: true\n")
		Out.Printf("    api_token: \"your-tempo-api-token\"\n")
		Out.Printf("═══════════════════════════════════════════\n")
	}

	return nil
}

func showTodaySummary(store *storage.Storage, jiraClient *jira.Client, tempoClient *tempo.Client, cfg *config.Config) error {
	Out.Printf("═══════════════════════════════════════════\n")
	Out.Printf("📊 Today's Time Tracking Summary\n")
	Out.Printf("═══════════════════════════════════════════\n")

	// Get current user for filtering
	currentUser, err := jiraClient.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Fetch from Tempo as source of truth
	Out.Debug("Fetching today's worklogs from Tempo")
	tempoWorklogs, tempoErr := tempoClient.GetTodayWorklogs(currentUser.AccountID)
	if tempoErr != nil {
		return fmt.Errorf("failed to fetch Tempo worklogs: %w", tempoErr)
	}

	// Get local entries
	localEntries, err := store.GetTodayEntries()
	if err != nil {
		return fmt.Errorf("failed to get local entries: %w", err)
	}

	// Calculate totals
	var tempoTotal, localTotal int

	for _, wl := range tempoWorklogs {
		tempoTotal += wl.TimeSpentSeconds
	}

	for _, entry := range localEntries {
		localTotal += entry.TimeSpentSeconds
	}

	// Display Tempo worklogs (source of truth)
	Out.Success(fmt.Sprintf("Tempo Worklogs (%d entries): %s", len(tempoWorklogs), timeparse.Format(tempoTotal)))
	if len(tempoWorklogs) > 0 {
		for _, wl := range tempoWorklogs {
			Out.Printf("  %s - %-10s [%-12s] %s\n",
				wl.StartTime,
				timeparse.Format(wl.TimeSpentSeconds),
				wl.Description,
				wl.IssueKey,
			)
		}
	}

	// Display local cache section
	Out.Printf("\n📦 Local Cache (%d entries): %s\n", len(localEntries), timeparse.Format(localTotal))
	if len(localEntries) > 0 {
		for _, entry := range localEntries {
			syncStatus := ""
			syncInfo := ""

			if entry.SyncedToJira && entry.SyncedToTempo {
				syncStatus = "✓"
				syncInfo = "Synced"
			} else if entry.SyncedToJira && !entry.SyncedToTempo {
				syncStatus = "⚠"
				syncInfo = "Jira only"
			} else if !entry.SyncedToJira && entry.SyncedToTempo {
				syncStatus = "⚠"
				syncInfo = "Tempo only"
			} else {
				syncStatus = "✗"
				syncInfo = "Not synced"
			}

			Out.Printf("  %s %s - %-10s [%-12s] %s (%s)\n",
				syncStatus,
				entry.Started.Format("15:04"),
				entry.TimeSpent,
				entry.Label,
				entry.IssueKey,
				syncInfo,
			)
		}
	}

	Out.Printf("\n═══════════════════════════════════════════\n")

	// Show comparison between Tempo and local data
	if len(localEntries) > 0 {
		diff := tempoTotal - localTotal
		if diff == 0 {
			Out.Success("Local cache matches Tempo")
		} else if diff > 0 {
			Out.Warn(fmt.Sprintf("Tempo has %s more than local cache", timeparse.Format(diff)))
		} else {
			Out.Warn(fmt.Sprintf("Local cache has %s not synced to Tempo", timeparse.Format(-diff)))
		}
	}

	Out.Println("═══════════════════════════════════════════")

	return nil
}
