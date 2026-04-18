package pipeline

import (
	"database/sql"
	"fmt"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/discover"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/restore"
)

// RunOptions configures a full batch run.
type RunOptions struct {
	Client       github.Client
	RestoreFrom  string   // if set, rebuild DB from this data/ tree before fetch
	Seeds        []string // "owner/name" entries
	Today        string   // YYYY-MM-DD for daily_stars snapshot
	UpdatedAt    string   // ISO-8601 embedded in JSON
	ComputedDate string   // YYYY-MM-DD for rankings row key (usually == Today)
	OutDir       string   // JSON output root (parent of repos/)
	TopN         int      // cap per period (<= 0 = all)
	SkipDiscover bool     // skip search-based discovery
	MaxPages     int      // pages per discover query (default 1 when unset)
}

// RunAll executes fetch → optional discover → compute → export. Any stage
// returning an error aborts the run; partial DB writes are already committed
// per-stage, which is by design (idempotent re-runs).
func RunAll(d *sql.DB, opts RunOptions) error {
	if err := db.Migrate(d); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// 0. restore history from data/ if requested (source-of-truth recovery).
	//    Lets GitHub Actions start with a fresh DB and still retain history.
	if opts.RestoreFrom != "" {
		if _, err := restore.FromDir(d, opts.RestoreFrom); err != nil {
			return fmt.Errorf("restore: %w", err)
		}
	}

	// 1. fetch seeds
	if len(opts.Seeds) > 0 {
		stats, err := fetch.Seeds(opts.Client, d, opts.Seeds, opts.Today)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		_ = stats // stats surfaced by cmd layer logs
	}

	// 2. discover (optional)
	if !opts.SkipDiscover {
		maxPages := opts.MaxPages
		if maxPages <= 0 {
			maxPages = 1
		}
		if _, err := discover.Run(opts.Client, d, discover.BuildQueries(), opts.Today, maxPages); err != nil {
			return fmt.Errorf("discover: %w", err)
		}
	}

	// 3. compute rankings + invariants
	if err := Compute(d, opts.ComputedDate); err != nil {
		return fmt.Errorf("compute: %w", err)
	}

	// 4. export JSON
	if err := export.Export(d, opts.OutDir, opts.UpdatedAt, opts.ComputedDate, opts.TopN); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}
