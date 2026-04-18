package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/github"
)

var fetchSeedFile string

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch seed repositories and record today's star counts",
	RunE:  runFetch,
}

func init() {
	fetchCmd.Flags().StringVar(&fetchSeedFile, "seed-file", "seeds.txt", "seed list (one owner/name per line)")
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(_ *cobra.Command, _ []string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}
	body, err := os.ReadFile(fetchSeedFile)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	seeds, err := fetch.ParseSeedsText(string(body))
	if err != nil {
		return fmt.Errorf("parse seeds: %w", err)
	}

	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		return err
	}

	client := github.NewAPIClient(token)
	today := time.Now().UTC().Format("2006-01-02")
	stats, err := fetch.Seeds(client, d, seeds, today)
	if err != nil {
		return err
	}
	log.Printf("fetch: %+v", stats)
	return nil
}
