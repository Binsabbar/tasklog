package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		logger := log.Logger.With().Str("component", "version").Logger()
		logger.Info().
			Str("commit", commit).
			Str("built_at", date).
			Str("built_by", builtBy).
			Msg("tasklog version information")
	},
}

func init() {
	rootCmd.AddCommand(VersionCmd)
}
