package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/pipeline"
	"github.com/kotenbu135/starise/batch/internal/translate"
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
	runTranslateProvider  string
	runTranslateCacheDir  string
	runTranslateLimit     int
	runTranslateBatchSize int
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
			AllowEmptyRankings:  runAllowEmptyRankings,
			TranslationCacheDir: runTranslateCacheDir,
			TranslateLimit:      runTranslateLimit,
			TranslateBatchSize:  runTranslateBatchSize,
		}
		if tr, err := buildRunTranslator(); err != nil {
			fmt.Printf("run: translate disabled: %v\n", err)
		} else {
			opts.Translator = tr
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
	runCmd.Flags().StringVar(&runTranslateProvider, "translate-provider", "gemini",
		"translation provider for description_ja: claude | gemini | none")
	runCmd.Flags().StringVar(&runTranslateCacheDir, "translate-cache-dir", "../data/translations",
		"translation cache root; empty = disable cache reads/writes entirely")
	runCmd.Flags().IntVar(&runTranslateLimit, "translate-limit", 1000,
		"cap new translations per run (free-tier safety; 0 = unlimited)")
	runCmd.Flags().IntVar(&runTranslateBatchSize, "translate-batch-size", 50,
		"strings per provider call")
}

// buildRunTranslator returns the configured Translator, or nil + reason
// when translation should be skipped (no API key, "none", etc).
func buildRunTranslator() (translate.Translator, error) {
	switch strings.ToLower(runTranslateProvider) {
	case "none", "off", "":
		return nil, fmt.Errorf("translate-provider=none")
	case "claude":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return &translate.ClaudeTranslator{APIKey: key}, nil
	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not set")
		}
		return &translate.GeminiTranslator{APIKey: key}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", runTranslateProvider)
	}
}
