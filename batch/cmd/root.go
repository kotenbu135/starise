package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var dbPath string

var rootCmd = &cobra.Command{
	Use:   "starise-batch",
	Short: "starise batch processor — fetch GitHub stars and compute rankings",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "starise.db", "SQLite database path")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
