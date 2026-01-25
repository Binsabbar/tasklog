package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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
	commentFlag  string
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
	logCmd.Flags().StringVarP(&commentFlag, "message", "m", "", "Description/comment for the time entry")
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
			fmt.Fprintf(cmd.OutOrStderr(), "  %-15s %s%s\n", sc.Name, sc.Task, timeInfo)
		}
		fmt.Fprintf(cmd.OutOrStderr(), "\n")
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Flags:\n")
	fmt.Fprintf(cmd.OutOrStderr(), "%s", cmd.Flags().FlagUsages())

	return nil
}

func runLog(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := checkConfig()
	if err != nil {
		return err
	}

	// Check if first argument is a shortcut name
	var shortcut *config.ShortcutEntry
	if len(args) > 0 {
		shortcutName = args[0]
		var found bool
		shortcut, found = cfg.GetShortcut(shortcutName)
		if !found {
			return fmt.Errorf("shortcut '%s' not found in configuration", shortcutName)
		}
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

	// Check if using a shortcut
	if shortcut != nil {
		log.Debug().Str("shortcut", shortcut.Name).Msg("Using shortcut")

		// Use shortcut values
		if taskKey == "" {
			taskKey = shortcut.Task
		}
		if timeSpent == "" && shortcut.Time != "" {
			timeSpent = shortcut.Time
		}
		if commentFlag == "" && shortcut.Comment != "" {
			commentFlag = shortcut.Comment
		}
	}

	// Get task
	if taskKey != "" {
		// CLI mode: fail fast, no retry
		// User explicitly provided a task key via flag, so if it's wrong they should fix the command
		log.Debug().Str("task", taskKey).Msg("Fetching specified task")
		issue, err := jiraClient.GetIssue(taskKey)
		if err != nil {
			return fmt.Errorf("failed to fetch task %s: %w", taskKey, err)
		}
		selectedIssue = issue
		fmt.Printf("Task: %s - %s\n", selectedIssue.Key, selectedIssue.Fields.Summary)
	} else {
		// Interactive task selection
		log.Debug().Msg("Fetching in-progress tasks")
		inProgressIssues, err := jiraClient.GetInProgressIssues(cfg.Jira.TaskStatuses)
		if err != nil {
			return fmt.Errorf("failed to fetch in-progress tasks: %w", err)
		}

		selectedIssue, err = ui.SelectTask(inProgressIssues)
		if err != nil {
			return fmt.Errorf("failed to select task: %w", err)
		}

		// If user chose to search, perform the search with retry
		if selectedIssue.Fields.Summary == "" {
			for {
				searchResults, err := jiraClient.SearchIssues(selectedIssue.Key)
				if err != nil {
					return fmt.Errorf("failed to search tasks: %w", err)
				}

				// Check if search returned results
				if len(searchResults) == 0 {
					fmt.Printf("No tasks found for '%s'\n", selectedIssue.Key)
					fmt.Println("(Press Ctrl+C to cancel)")

					// Prompt to search again
					newIssue, err := ui.PromptTaskSearch()
					if err != nil {
						return fmt.Errorf("failed to get search key: %w", err)
					}
					selectedIssue = newIssue
					continue
				}

				// Results found, let user select
				selectedIssue, err = ui.SelectFromSearchResults(searchResults)
				if err != nil {
					return fmt.Errorf("failed to select from search results: %w", err)
				}
				break
			}

			// Fetch full issue details
			selectedIssue, err = fetchTaskWithRetry(jiraClient, selectedIssue.Key)
			if err != nil {
				return err
			}
		}
	}

	// Get time spent
	if timeSpent != "" {
		timeSeconds, err = timeparse.Parse(timeSpent)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
	} else {
		timeSeconds, err = ui.PromptTimeSpent()
		if err != nil {
			return fmt.Errorf("failed to get time spent: %w", err)
		}
	}

	// Get required comment/description
	var comment string
	if commentFlag != "" {
		comment = commentFlag
	} else {
		var err error
		comment, err = ui.PromptComment()
		if err != nil {
			return fmt.Errorf("failed to get comment: %w", err)
		}
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
		started, err = ui.PromptStartTime(timeSeconds)
		if err != nil {
			return fmt.Errorf("failed to get start time: %w", err)
		}
	} else {
		// Shortcut or CLI mode without --at: default to ending now
		started = time.Now().Add(-time.Duration(timeSeconds) * time.Second)
	}

	// Confirm before logging
	fmt.Printf("\n")
	fmt.Printf("Task:        %s - %s\n", selectedIssue.Key, selectedIssue.Fields.Summary)
	fmt.Printf("Time:        %s\n", timeparse.Format(timeSeconds))
	fmt.Printf("Started:     %s\n", started.Format("Mon Jan 2 15:04"))
	fmt.Printf("Description: %s\n", comment)
	fmt.Printf("\n")

	confirmed, err := ui.Confirm("Log this time entry?")
	if err != nil {
		return fmt.Errorf("failed to confirm: %w", err)
	}

	if !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	// Create time entry
	entry := &storage.TimeEntry{
		IssueKey:         selectedIssue.Key,
		IssueSummary:     selectedIssue.Fields.Summary,
		TimeSpentSeconds: timeSeconds,
		TimeSpent:        timeparse.Format(timeSeconds),
		Comment:          comment,
		Started:          started,
		SyncedToJira:     false,
		SyncedToTempo:    false,
	}

	// Save to local storage first
	if err := store.AddTimeEntry(entry); err != nil {
		return fmt.Errorf("failed to save time entry locally: %w", err)
	}

	fmt.Println("✓ Saved to local cache")

	// Log to Jira
	log.Debug().Msg("Logging to Jira")
	worklog, err := jiraClient.AddWorklog(selectedIssue.Key, timeSeconds, started, comment)
	if err != nil {
		log.Error().Err(err).Msg("Failed to log to Jira")
		fmt.Printf("⚠ Failed to log to Jira: %v\n", err)
	} else {
		entry.SyncedToJira = true
		entry.JiraWorklogID = &worklog.ID
		fmt.Println("✓ Logged to Jira")

		// If Tempo is enabled, Jira automatically creates a Tempo worklog
		// Mark as synced to Tempo since it's handled by Jira
		if cfg.Tempo.Enabled {
			entry.SyncedToTempo = true
			fmt.Println("✓ Tempo worklog created automatically by Jira")
		}
	}

	// Mark as synced if Tempo is not enabled
	if !cfg.Tempo.Enabled {
		entry.SyncedToTempo = true
	}

	// Update storage with sync status
	if err := store.UpdateTimeEntry(entry); err != nil {
		log.Error().Err(err).Msg("Failed to update time entry sync status")
	}

	// Show today's summary
	fmt.Println()
	if cfg.Tempo.Enabled && cfg.Tempo.APIToken != "" {
		if err := showTodaySummary(store, jiraClient, tempoClient, cfg); err != nil {
			log.Error().Err(err).Msg("Failed to show summary")
		}
	} else {
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println("📊 Summary is disabled")
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println("To enable time tracking summary, configure Tempo API in your config:")
		fmt.Println("  tempo:")
		fmt.Println("    enabled: true")
		fmt.Println("    api_token: \"your-tempo-api-token\"")
		fmt.Println("═══════════════════════════════════════════")
	}

	return nil
}

// fetchTaskWithRetry attempts to fetch a task, retrying on "not found" errors.
// It distinguishes between "task not found" errors (which trigger retry) and
// other API errors like network failures or authentication issues (which exit immediately).
func fetchTaskWithRetry(jiraClient *jira.Client, initialTaskKey string) (*jira.Issue, error) {
	currentTaskKey := initialTaskKey

	for {
		issue, err := jiraClient.GetIssue(currentTaskKey)
		if err == nil {
			return issue, nil
		}

		// Check if it's a "not found" error (404) vs other API errors
		// If it's a network/auth error, don't retry
		if !isTaskNotFoundError(err) {
			return nil, fmt.Errorf("failed to fetch task %s: %w", currentTaskKey, err)
		}

		// Task not found - prompt to try again
		fmt.Printf("Error: Task %s not found\n", currentTaskKey)
		fmt.Println("(Press Ctrl+C to cancel)")

		newIssue, err := ui.PromptManualTaskKey()
		if err != nil {
			return nil, err // User cancelled with Ctrl+C
		}
		currentTaskKey = newIssue.Key
	}
}

// isTaskNotFoundError checks if the error is a 404 not found error.
// Returns true for "task not found" errors that should trigger retry,
// false for other errors (network, auth, etc.) that should exit immediately.
func isTaskNotFoundError(err error) bool {
	// Check error message for common "not found" patterns
	// The Jira client returns errors in format: "API request failed with status 404: ..."
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "404") ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "does not exist")
}

func showTodaySummary(store *storage.Storage, jiraClient *jira.Client, tempoClient *tempo.Client, cfg *config.Config) error {
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("📊 Today's Time Tracking Summary")
	fmt.Println("═══════════════════════════════════════════")

	// Get current user for filtering
	currentUser, err := jiraClient.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Fetch from Tempo as source of truth
	log.Debug().Msg("Fetching today's worklogs from Tempo")
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
	fmt.Printf("\n✓ Tempo Worklogs (%d entries): %s\n", len(tempoWorklogs), timeparse.Format(tempoTotal))
	if len(tempoWorklogs) > 0 {
		for _, wl := range tempoWorklogs {
			fmt.Printf("  %s - %-10s [%-12s] %s\n",
				wl.GetLocalStartTime(),
				timeparse.Format(wl.TimeSpentSeconds),
				wl.Description,
				wl.IssueKey(),
			)
		}
	}

	// Display local cache section
	fmt.Printf("\n📦 Local Cache (%d entries): %s\n", len(localEntries), timeparse.Format(localTotal))
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

			// Display format: Comment width increased from 12 to 20 to accommodate longer descriptions
			fmt.Printf("  %s %s - %-10s [%-20s] %s (%s)\n",
				syncStatus,
				entry.Started.Format("15:04"),
				entry.TimeSpent,
				entry.Comment,
				entry.IssueKey,
				syncInfo,
			)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════")

	// Show comparison between Tempo and local data
	if len(localEntries) > 0 {
		diff := tempoTotal - localTotal
		if diff == 0 {
			fmt.Println("✓ Local cache matches Tempo")
		} else if diff > 0 {
			fmt.Printf("⚠️  Tempo has %s more than local cache\n", timeparse.Format(diff))
		} else {
			fmt.Printf("⚠️  Local cache has %s not synced to Tempo\n", timeparse.Format(-diff))
		}
	}

	fmt.Println("═══════════════════════════════════════════")

	return nil
}
