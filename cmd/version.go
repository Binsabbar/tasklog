package cmd

import (
	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		Out.Println(GetVersion())
	},
}

func init() {
	rootCmd.AddCommand(VersionCmd)
}
