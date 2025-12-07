package main

import (
	"os"

	"tasklog/cmd"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version information set via ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	// Configure zerolog
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Check for debug log level from environment
	if os.Getenv("TASKLOG_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Set version information
	cmd.SetVersionInfo(version, commit, date, builtBy)
	cmd.SetCommandsVisibility()

	// Execute root command
	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute command")
	}
}
