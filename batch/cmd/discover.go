package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/github"
)

var discoverMaxPages int

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover repositories via GitHub Search API",
	RunE:  runDiscover,
}

func init() {
	discoverCmd.Flags().IntVar(&discoverMaxPages, "max-pages", 10, "pages per search query (100 repos/page)")
	rootCmd.AddCommand(discoverCmd)
}

func runDiscover(_ *cobra.Command, _ []string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
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
	stats, err := discover.Run(client, d, discover.BuildQueries(), today, discoverMaxPages)
	if err != nil {
		return err
	}
	log.Printf("discover: %+v", stats)
	return nil
}
