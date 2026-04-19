package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	runSeedFile           string
	runOutDir             string
	runRestoreFrom        string
	runTopN               int
	runMaxPages           int
	runQuery              string
	runUsePreset          bool
	runDiscoverConcurrency int
	runSkipDiscover       bool
	runSkipRefresh        bool
	runAllowEmptyRankings bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "All-in-one: restore -> fetch -> discover -> refresh -> compute -> export",
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
		owners, names, err := fetch.LoadSeeds(runSeedFile)
		if err != nil {
			return fmt.Errorf("seeds: %w", err)
		}
		t := today()
		if t == "" {
			t = time.Now().UTC().Format("2006-01-02")
		}
		now := time.Now().UTC().Format(time.RFC3339)
		opts := pipeline.Options{
			Client: c, Today: t,
			SeedOwners: owners, SeedNames: names,
			OutDir: runOutDir, RestoreFrom: runRestoreFrom,
			TopN: runTopN, MaxPages: runMaxPages,
			DiscoverConcurrency: runDiscoverConcurrency,
			SkipDiscover:        runSkipDiscover, SkipRefresh: runSkipRefresh,
			UpdatedAt: now, GeneratedAt: now,
			AllowEmptyRankings: runAllowEmptyRankings,
		}
		if runUsePreset {
			opts.SearchQueries = discover.BuildQuerySet(time.Now().UTC())
		} else {
			opts.SearchQuery = runQuery
		}
		report, err := pipeline.RunAll(context.Background(), d, opts)
		fmt.Printf("run: %+v\n", report)
		return err
	},
}

func init() {
	runCmd.Flags().StringVar(&runSeedFile, "seed-file", "seeds.txt", "seed list path")
	runCmd.Flags().StringVar(&runOutDir, "out-dir", "../data", "output directory")
	runCmd.Flags().StringVar(&runRestoreFrom, "restore-from", "../data", "restore source dir (empty = skip)")
	runCmd.Flags().IntVar(&runTopN, "top-n", 2000, "max rank entries per slot")
	runCmd.Flags().IntVar(&runMaxPages, "max-pages", 10, "discover search pages")
	runCmd.Flags().StringVar(&runQuery, "query", "stars:>10 sort:stars-desc", "discover search query (ignored when --preset is set)")
	runCmd.Flags().BoolVar(&runUsePreset, "preset", false,
		"use the built-in multi-query preset (star bands × language × topics). "+
			"Produces the v1-scale ~30k discovery sweep; overrides --query.")
	runCmd.Flags().IntVar(&runDiscoverConcurrency, "discover-concurrency", 5,
		"parallel Search API queries when --preset is set")
	runCmd.Flags().BoolVar(&runSkipDiscover, "skip-discover", false, "skip discover step")
	runCmd.Flags().BoolVar(&runSkipRefresh, "skip-refresh", false, "skip refresh step")
	runCmd.Flags().BoolVar(&runAllowEmptyRankings, "allow-empty-rankings", false,
		"do NOT abort when all 6 ranking slots are empty (required for bootstrap day "+
			"when data/ has no history yet — production CI sets this true so daily export "+
			"always proceeds; I12 still catches logic bugs in tests)")
}
