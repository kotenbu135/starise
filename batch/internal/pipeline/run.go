// Package pipeline orchestrates the full daily run: restore -> fetch ->
// discover -> refresh -> compute -> export -> cleanup.
package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/kotenbu135/starise/batch/internal/refresh"
	"github.com/kotenbu135/starise/batch/internal/restore"
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
	SearchQuery        string
	SkipDiscover       bool
	SkipRefresh        bool
	UpdatedAt          string
	GeneratedAt        string
	// AllowEmptyRankings disables the I12 macro check. Used by the multi-day
	// simulation harness where early days legitimately produce empty slots
	// (no history yet to rank against). Production runs must leave it false.
	AllowEmptyRankings bool
}

type RunReport struct {
	Restored    restore.Result
	Fetched     fetch.Result
	Discovered  discover.Result
	Refreshed   refresh.Result
	ExportRepos int
	Cleanup     export.CleanupResult
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

	if !opts.SkipDiscover && opts.SearchQuery != "" {
		res, err := discover.Run(ctx, d, opts.Client, github.SearchOptions{
			Query: opts.SearchQuery, MaxPages: opts.MaxPages, PerPage: 50,
		}, opts.Today)
		if err != nil {
			return report, fmt.Errorf("discover: %w", err)
		}
		report.Discovered = res
	}

	if !opts.SkipRefresh {
		res, err := refresh.Run(ctx, d, opts.Client, opts.Today, refresh.DefaultMaxFailureRate)
		if err != nil && !errors.Is(err, refresh.ErrFailureRateExceeded) {
			return report, fmt.Errorf("refresh: %w", err)
		}
		report.Refreshed = res
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

	if opts.OutDir != "" {
		written, err := export.Export(d, export.Options{
			OutDir: opts.OutDir, UpdatedAt: opts.UpdatedAt,
			GeneratedAt: opts.GeneratedAt, ComputedDate: opts.Today, TopN: opts.TopN,
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
