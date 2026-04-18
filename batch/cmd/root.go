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
	Short: "starise batch processor (v3, invariant-driven)",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "starise.db", "SQLite database path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose logging")

	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(refreshCmd)
	rootCmd.AddCommand(computeCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(runCmd)
}
