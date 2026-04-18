package cmd

import (
	"github.com/spf13/cobra"
)

var (
	dbPath  string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "starise",
	Short: "starise batch processor",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "starise.db", "SQLite database path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose logging")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
