package cmd

import (
	"fmt"
	"time"

	"tasklog/internal/config"
	"tasklog/internal/slack"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

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

Run without arguments to list available breaks.`,
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
		if len(cfg.Breaks) == 0 {
			fmt.Println("❌ No breaks configured. Add breaks to your config.yaml file.")
			fmt.Println("\nExample configuration:")
			fmt.Println("breaks:")
			fmt.Println("  - name: \"lunch\"")
			fmt.Println("    duration: 60")
			fmt.Println("    emoji: \":fork_and_knife:\"")
			return
		}

		fmt.Println("📋 Available breaks:")
		fmt.Println("")
		for _, b := range cfg.Breaks {
			emoji := b.Emoji
			if emoji == "" {
				emoji = "⏸️"
			}
			fmt.Printf("  %s %-12s - %d minutes\n", emoji, b.Name, b.Duration)
		}
		fmt.Println("\nUsage: tasklog break [break-name]")
		return
	}

	breakName := args[0]

	// Get break configuration
	breakEntry, found := cfg.GetBreak(breakName)
	if !found {
		log.Fatal().
			Str("break_name", breakName).
			Msg("Break not found in configuration. Please add it to your config.yaml")
	}

	// Check if Slack is configured
	if cfg.Slack.UserToken == "" || cfg.Slack.ChannelID == "" {
		log.Warn().Msg("Slack not configured. Break registered but Slack status not updated.")
		fmt.Printf("⏸️  Taking a %s break for %d minutes\n", breakName, breakEntry.Duration)
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
		statusEmoji = ":double_vertical_bar:"
	}

	// Add 5 minutes buffer to auto-clear the status
	statusExpirationMinutes := breakEntry.Duration + 5

	err = slackClient.SetStatus(statusText, statusEmoji, statusExpirationMinutes)
	if err != nil {
		log.Error().Err(err).Str("emoji", statusEmoji).Msg("Failed to update Slack status")

		// If the error is about invalid emoji and we're not already using the default, retry with default
		if statusEmoji != ":double_vertical_bar:" &&
			(err.Error() == "slack API error: profile_status_set_failed_not_valid_emoji" ||
				err.Error() == "slack API error: profile_status_set_failed_not_emoji_syntax" ||
				err.Error() == "slack API error: invalid_emoji") {
			log.Warn().Msg("Invalid emoji detected, retrying with default emoji")
			err = slackClient.SetStatus(statusText, ":double_vertical_bar:", statusExpirationMinutes)
			if err != nil {
				log.Error().Err(err).Msg("Failed to update Slack status with default emoji")
			} else {
				log.Info().
					Str("status", statusText).
					Str("emoji", ":double_vertical_bar:").
					Int("expiration_minutes", statusExpirationMinutes).
					Msg("Slack status updated with default emoji")
				statusUpdated = true
			}
		}
	} else {
		log.Info().
			Str("status", statusText).
			Str("emoji", statusEmoji).
			Int("expiration_minutes", statusExpirationMinutes).
			Msg("Slack status updated")
		statusUpdated = true
	}

	// Post message to channel
	emojiForMessage := breakEntry.Emoji
	if emojiForMessage == "" {
		emojiForMessage = ":double_vertical_bar:"
	}
	message := fmt.Sprintf("🔔 Taking a %s *%s break* — Back in %d minutes at *%s*",
		emojiForMessage,
		breakName,
		breakEntry.Duration,
		returnTime.Format("3:04 PM"))

	err = slackClient.PostMessage(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to post message to Slack")
	} else {
		log.Info().
			Str("channel", cfg.Slack.ChannelID).
			Str("message", message).
			Msg("Message posted to Slack")
		messagePosted = true
	}

	// Display success message with accurate status
	fmt.Printf("✅ Break registered: %s (%d minutes)\n", breakName, breakEntry.Duration)
	fmt.Printf("📅 Return time: %s\n", returnTime.Format("3:04 PM"))

	if statusUpdated && messagePosted {
		fmt.Printf("💬 Slack updated: Status set and message posted\n")
	} else if messagePosted {
		fmt.Printf("💬 Slack updated: Message posted (status not updated)\n")
	} else if statusUpdated {
		fmt.Printf("💬 Slack updated: Status set (message failed)\n")
	} else {
		fmt.Printf("⚠️  Slack update failed\n")
	}
}
