package cmd

import (
	"fmt"
	"os"
	"time"

	"tasklog/internal/config"
	"tasklog/internal/slack"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const defaultBreakEmoji = ":double_vertical_bar:"

var breakCmd = &cobra.Command{
	Use:   "break [break-name]",
	Short: "Register a break and update Slack status",
	Long: `Register a break (e.g., lunch, prayer, coffee) and automatically:
- Update your Slack status with break emoji
- Post a message in the configured Slack channel
- Set status to expire after break duration

Example:
  tasklog break lunch
  tasklog break prayer
  tasklog break coffee

Run without arguments to list available breaks.` + configHelp,
	Args: cobra.MaximumNArgs(1),
	Run:  runBreak,
}

func init() {
	rootCmd.AddCommand(breakCmd)
}

func runBreak(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// If no break name provided, list available breaks
	if len(args) == 0 {
		if len(cfg.Slack.Breaks) == 0 {
			Out.Error(fmt.Errorf("no breaks configured. Add breaks to your config.yaml file"))
			Out.Println("\nExample configuration:")
			Out.Println("breaks:")
			Out.Println("  - name: \"lunch\"")
			Out.Println("    duration: 60")
			Out.Println("    emoji: \":fork_and_knife:\"")
			return
		}

		Out.Println("📋 Available breaks:")
		Out.Println("")
		for _, b := range cfg.Slack.Breaks {
			emoji := b.Emoji
			if emoji == "" {
				emoji = "⏸️"
			}
			Out.Printf("  %s %-12s - %d minutes\n", emoji, b.Name, b.Duration)
		}
		Out.Println("\nUsage: tasklog break [break-name]")
		return
	}

	breakName := args[0]

	// Get break configuration
	breakEntry, found := cfg.GetBreak(breakName)
	if !found {
		Out.Error(fmt.Errorf("break '%s' not found in configuration. Please add it to your config.yaml", breakName))
		os.Exit(1)
	}

	// Check if Slack is configured
	if cfg.Slack.UserToken == "" || cfg.Slack.ChannelID == "" {
		Out.Warn("Slack not configured. Break registered but Slack status not updated.")
		Out.Printf("⏸️  Taking a %s break for %d minutes\n", breakName, breakEntry.Duration)
		return
	}

	// Create Slack client
	slackClient := slack.NewClient(cfg.Slack.UserToken, cfg.Slack.ChannelID)

	// Calculate return time
	returnTime := time.Now().Add(time.Duration(breakEntry.Duration) * time.Minute)

	// Track what succeeded
	statusUpdated := false
	messagePosted := false

	// Set Slack status with 5 extra minutes buffer for auto-clear
	statusText := fmt.Sprintf("On %s break (back at %s)", breakName, returnTime.Format("3:04 PM"))
	statusEmoji := breakEntry.Emoji
	if statusEmoji == "" {
		statusEmoji = defaultBreakEmoji
	}

	// Add 5 minutes buffer to auto-clear the status
	statusExpirationMinutes := breakEntry.Duration + 5

	err = slackClient.SetStatus(statusText, statusEmoji, statusExpirationMinutes)
	if err != nil {
		Out.Error(fmt.Errorf("failed to update Slack status (emoji: %s): %w", statusEmoji, err))

		// If the error is about invalid emoji and we're not already using the default, retry with default
		if statusEmoji != defaultBreakEmoji &&
			(err.Error() == "slack API error: profile_status_set_failed_not_valid_emoji" ||
				err.Error() == "slack API error: profile_status_set_failed_not_emoji_syntax" ||
				err.Error() == "slack API error: invalid_emoji") {
			Out.Warn("Invalid emoji detected, retrying with default emoji")
			err = slackClient.SetStatus(statusText, defaultBreakEmoji, statusExpirationMinutes)
			if err != nil {
				Out.Error(fmt.Errorf("failed to update Slack status with default emoji: %w", err))
			} else {
				Out.Info(fmt.Sprintf("Slack status updated with default emoji: %s", statusText))
				statusUpdated = true
			}
		}
	} else {
		Out.Info(fmt.Sprintf("Slack status updated: %s", statusText))
		statusUpdated = true
	}

	// Post message to channel
	emojiForMessage := breakEntry.Emoji
	if emojiForMessage == "" {
		emojiForMessage = defaultBreakEmoji
	}
	message := fmt.Sprintf("🔔 Taking a %s *%s break* — Back in %d minutes at *%s*",
		emojiForMessage,
		breakName,
		breakEntry.Duration,
		returnTime.Format("3:04 PM"))

	err = slackClient.PostMessage(message)
	if err != nil {
		Out.Error(fmt.Errorf("failed to post message to Slack: %w", err))
	} else {
		Out.Info("Message posted to Slack")
		messagePosted = true
	}

	// Display success message with accurate status
	Out.Success(fmt.Sprintf("Break registered: %s (%d minutes)", breakName, breakEntry.Duration))
	Out.Printf("📅 Return time: %s\n", returnTime.Format("3:04 PM"))

	if statusUpdated && messagePosted {
		Out.Printf("💬 Slack updated: Status set and message posted\n")
	} else if messagePosted {
		Out.Printf("💬 Slack updated: Message posted (status not updated)\n")
	} else if statusUpdated {
		Out.Printf("💬 Slack updated: Status set (message failed)\n")
	} else {
		Out.Warn("Slack update failed")
	}
}
