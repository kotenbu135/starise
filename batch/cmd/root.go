package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
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

// openDB is shared by every subcommand.
func openDB() (*sql.DB, error) {
	d, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", dbPath, err)
	}
	return d, nil
}

// newClient builds the production GraphQL client from GITHUB_TOKEN.
func newClient() (github.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN not set")
	}
	return github.NewGraphQLClient(token), nil
}

func today() string {
	return os.Getenv("STARISE_TODAY")
}
