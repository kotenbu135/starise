package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/spf13/cobra"
)

var (
	discoverMaxPages int
	discoverQuery    string
	discoverPerPage  int
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover new repositories via Search API",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		c, err := newClient()
		if err != nil {
			return err
		}
		t := today()
		if t == "" {
			t = time.Now().UTC().Format("2006-01-02")
		}
		opts := github.SearchOptions{Query: discoverQuery, MaxPages: discoverMaxPages, PerPage: discoverPerPage}
		res, err := discover.Run(context.Background(), d, c, opts, t)
		if err != nil {
			return err
		}
		fmt.Printf("discover: discovered=%d refreshed=%d errors=%d\n", res.Discovered, res.Refreshed, res.Errors)
		return nil
	},
}

func init() {
	discoverCmd.Flags().IntVar(&discoverMaxPages, "max-pages", 10, "max search pages")
	discoverCmd.Flags().IntVar(&discoverPerPage, "per-page", 50, "results per page")
	discoverCmd.Flags().StringVar(&discoverQuery, "query", "stars:>10 sort:stars-desc", "GitHub search query")
}
