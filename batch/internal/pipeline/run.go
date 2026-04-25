// Package pipeline orchestrates the full daily run: restore -> fetch ->
// discover -> refresh -> compute -> export -> cleanup.
package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/kotenbu135/starise/batch/internal/refresh"
	"github.com/kotenbu135/starise/batch/internal/restore"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

type Options struct {
	Client             github.Client
	Today              string // YYYY-MM-DD
	Seeds              []string // owner/name pairs as one slice; convenience parser below
	SeedOwners         []string
	SeedNames          []string
	OutDir             string
	RestoreFrom        string // empty = skip restore
	TopN               int
	MaxPages           int
	// SearchQuery is the legacy single-query entrypoint. When SearchQueries
	// is non-empty it takes precedence.
	SearchQuery        string
	// SearchQueries runs the multi-query discover path (star bands × language
	// × topic) in parallel. When set, SearchQuery is ignored.
	SearchQueries      []string
	// DiscoverConcurrency caps parallel Search API queries. 0 defaults to 1
	// (sequential). 5 is a safe production setting: well below GitHub's
	// secondary rate limit and leaves headroom for refresh running after.
	DiscoverConcurrency int
	SkipDiscover       bool
	SkipRefresh        bool
	UpdatedAt          string
	GeneratedAt        string
	// AllowEmptyRankings disables the I12 macro check. Used by the multi-day
	// simulation harness where early days legitimately produce empty slots
	// (no history yet to rank against). Production runs must leave it false.
	AllowEmptyRankings bool

	// Translator is the optional translation provider for description_ja.
	// When nil, the translate step is skipped — Export still reads from
	// any pre-existing TranslationCacheDir, so prior runs' translations
	// remain usable.
	Translator translate.Translator
	// TranslateLimit caps new translations performed in this run; protects
	// the daily Gemini free-tier quota.
	TranslateLimit int
	// TranslateBatchSize is strings per provider call. 0 → 32.
	TranslateBatchSize int
	// TranslationCacheDir is the on-disk cache root (e.g. data/translations).
	// Required for translation to do anything; also passed to export.
	TranslationCacheDir string
}

type RunReport struct {
	Restored       restore.Result
	Fetched        fetch.Result
	Discovered     discover.Result
	DiscoveredMany discover.ManyResult
	Refreshed      refresh.Result
	Translated     translate.RunStats
	ExportRepos    int
	Cleanup        export.CleanupResult
}

// RunAll executes the full pipeline. Returns a report and any error from a
// step that should abort the run (the only soft-error tolerated mid-run is
// per-repo fetch/discover failures already counted in the result struct).
func RunAll(ctx context.Context, d *sql.DB, opts Options) (RunReport, error) {
	if opts.Today == "" {
		opts.Today = time.Now().UTC().Format("2006-01-02")
	}
	if opts.UpdatedAt == "" {
		opts.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if opts.GeneratedAt == "" {
		opts.GeneratedAt = opts.UpdatedAt
	}
	if opts.TopN <= 0 {
		opts.TopN = 2000
	}

	report := RunReport{}

	if opts.RestoreFrom != "" {
		res, err := restore.FromDir(d, opts.RestoreFrom)
		if err != nil {
			// Not fatal — fresh data dir is a valid first-run state.
			// We still record what we got.
		}
		report.Restored = res
	}

	if len(opts.SeedOwners) > 0 {
		res, err := fetch.Run(ctx, d, opts.Client, opts.SeedOwners, opts.SeedNames, opts.Today)
		if err != nil {
			return report, fmt.Errorf("fetch: %w", err)
		}
		report.Fetched = res
	}

	if !opts.SkipDiscover {
		switch {
		case len(opts.SearchQueries) > 0:
			res, err := discover.RunMany(ctx, d, opts.Client, opts.SearchQueries, opts.Today, discover.RunManyOptions{
				Concurrency: opts.DiscoverConcurrency,
				MaxPages:    opts.MaxPages,
				PerPage:     100,
			})
			if err != nil {
				return report, fmt.Errorf("discover: %w", err)
			}
			report.DiscoveredMany = res
		case opts.SearchQuery != "":
			res, err := discover.Run(ctx, d, opts.Client, github.SearchOptions{
				Query: opts.SearchQuery, MaxPages: opts.MaxPages, PerPage: 50,
			}, opts.Today)
			if err != nil {
				return report, fmt.Errorf("discover: %w", err)
			}
			report.Discovered = res
		}
	}

	if !opts.SkipRefresh {
		res, err := refresh.Run(ctx, d, opts.Client, opts.Today, refresh.DefaultMaxFailureRate)
		// Capture telemetry (CostTotal / MinRemaining / Refreshed count)
		// BEFORE surfacing any error so the run summary log shows exactly
		// where a failing run got to — regression guard against the
		// 2026-04-20 incident where Refreshed:0 hid 32 batches of partial
		// data that had already been persisted to DB.
		report.Refreshed = res
		if err != nil && !errors.Is(err, refresh.ErrFailureRateExceeded) {
			return report, fmt.Errorf("refresh: %w", err)
		}
		if errors.Is(err, refresh.ErrFailureRateExceeded) {
			return report, fmt.Errorf("refresh: %w (rate=%v)", err, res.FailureRate)
		}
	}

	if opts.AllowEmptyRankings {
		if err := ranking.Compute(d, opts.Today, opts.TopN); err != nil {
			return report, fmt.Errorf("ranking: %w", err)
		}
	} else {
		if err := ranking.ComputeAndCheck(d, opts.Today, opts.TopN); err != nil {
			return report, fmt.Errorf("ranking: %w", err)
		}
	}

	// Translate descriptions before export so newly-translated entries are
	// picked up in this run's RepoDetail JSON. Failures are recorded in
	// stats and tolerated — Export will fall back to empty description_ja
	// (the frontend then renders the English original).
	if opts.Translator != nil && opts.TranslationCacheDir != "" {
		repos, err := db.ListAllRepositories(d)
		if err != nil {
			return report, fmt.Errorf("translate: list repos: %w", err)
		}
		descs := make([]string, 0, len(repos))
		for _, r := range repos {
			if r.Description != "" {
				descs = append(descs, r.Description)
			}
		}
		runner := &translate.Runner{
			Cache:      &translate.Cache{Dir: opts.TranslationCacheDir},
			Translator: opts.Translator,
			BatchSize:  opts.TranslateBatchSize,
		}
		stats, err := runner.Run(ctx, descs, opts.TranslateLimit)
		if err != nil {
			return report, fmt.Errorf("translate: %w", err)
		}
		report.Translated = stats
	}

	if opts.OutDir != "" {
		written, err := export.Export(d, export.Options{
			OutDir: opts.OutDir, UpdatedAt: opts.UpdatedAt,
			GeneratedAt: opts.GeneratedAt, ComputedDate: opts.Today, TopN: opts.TopN,
			TranslationCacheDir: opts.TranslationCacheDir,
		})
		if err != nil {
			return report, fmt.Errorf("export: %w", err)
		}
		report.ExportRepos = written

		cl, err := export.Cleanup(d, opts.OutDir, opts.Today)
		if err != nil {
			return report, fmt.Errorf("cleanup: %w", err)
		}
		report.Cleanup = cl
	}

	return report, nil
}
