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
	if cfg.Slack.BotToken == "" || cfg.Slack.ChannelID == "" {
		log.Warn().Msg("Slack not configured. Break registered but Slack status not updated.")
		fmt.Printf("⏸️  Taking a %s break for %d minutes\n", breakName, breakEntry.Duration)
		return
	}

	// Create Slack client
	slackClient := slack.NewClient(cfg.Slack.BotToken, cfg.Slack.ChannelID)

	// Calculate return time
	returnTime := time.Now().Add(time.Duration(breakEntry.Duration) * time.Minute)

	// Set Slack status
	statusText := fmt.Sprintf("On %s break (back at %s)", breakName, returnTime.Format("3:04 PM"))
	statusEmoji := breakEntry.Emoji
	if statusEmoji == "" {
		statusEmoji = ":pause_button:"
	}

	err = slackClient.SetStatus(statusText, statusEmoji, breakEntry.Duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update Slack status")
	} else {
		log.Info().
			Str("status", statusText).
			Str("emoji", statusEmoji).
			Msg("Slack status updated")
	}

	// Post message to channel
	message := fmt.Sprintf("🔔 Taking a *%s break* — Back in %d minutes at *%s*",
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
	}

	// Display success message
	fmt.Printf("✅ Break registered: %s (%d minutes)\n", breakName, breakEntry.Duration)
	fmt.Printf("📅 Return time: %s\n", returnTime.Format("3:04 PM"))
	fmt.Printf("💬 Slack updated: Status set and message posted\n")
}
