package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/spf13/cobra"
)

var fetchSeedFile string

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch seed repositories and write today's snapshot",
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
		owners, names, err := fetch.LoadSeeds(fetchSeedFile)
		if err != nil {
			return fmt.Errorf("seeds: %w", err)
		}
		t := today()
		if t == "" {
			t = time.Now().UTC().Format("2006-01-02")
		}
		res, err := fetch.Run(context.Background(), d, c, owners, names, t)
		if err != nil {
			return err
		}
		fmt.Printf("fetch: fetched=%d missing=%d errors=%d\n", res.Fetched, res.Missing, res.Errors)
		return nil
	},
}

func init() {
	fetchCmd.Flags().StringVar(&fetchSeedFile, "seed-file", "seeds.txt", "seed list path")
}
