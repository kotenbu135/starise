// Package refresh re-fetches today's star count for every non-deleted repo
// using the bulk nodes() GraphQL endpoint. 404 responses become soft deletes;
// archive flips are written back; if the failure rate exceeds the threshold
// the caller is signalled so the pipeline can abort (I4).
package refresh

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// Default failure-rate threshold per issue #2 I4.
const DefaultMaxFailureRate = 0.30

// ErrFailureRateExceeded indicates more than MaxFailureRate of refresh
// requests came back missing. Pipeline callers should exit non-zero.
var ErrFailureRateExceeded = errors.New("refresh: failure rate exceeded threshold")

type Result struct {
	Refreshed       int
	SoftDeleted     int
	ArchivedFlipped int
	FailureRate     float64
}

// Run loads all non-deleted repos, asks the client for fresh data via
// BulkRefresh, writes the new snapshot, soft-deletes 404s, updates archive
// flips. If the missing/total ratio exceeds maxFailureRate the call returns
// the result alongside ErrFailureRateExceeded.
func Run(ctx context.Context, d *sql.DB, c github.Client, today string, maxFailureRate float64) (Result, error) {
	repos, err := db.ListNonDeletedRepositories(d)
	if err != nil {
		return Result{}, fmt.Errorf("list non-deleted: %w", err)
	}
	if len(repos) == 0 {
		return Result{}, nil
	}

	idToRepo := make(map[string]db.Repository, len(repos))
	ids := make([]string, 0, len(repos))
	for _, r := range repos {
		ids = append(ids, r.GitHubID)
		idToRepo[r.GitHubID] = r
	}

	// Partial-data contract: BulkRefresh may return found/missing from the
	// batches that succeeded alongside an error from one that didn't. We
	// persist whatever came back before surfacing the error so a single
	// rate-limit blip doesn't invalidate 29k other snapshots.
	found, missing, _, bulkErr := c.BulkRefresh(ctx, ids)

	res := Result{}

	for _, fresh := range found {
		prev, ok := idToRepo[fresh.GitHubID]
		if !ok {
			// Should not happen — bulk returned an unknown id.
			continue
		}
		id, err := db.UpsertRepository(d, db.Repository{
			GitHubID: fresh.GitHubID, Owner: fresh.Owner, Name: fresh.Name,
			Description: fresh.Description, URL: fresh.URL, HomepageURL: fresh.HomepageURL,
			Language: fresh.Language, License: fresh.License, Topics: fresh.Topics,
			IsArchived: fresh.IsArchived, IsFork: fresh.IsFork, ForkCount: fresh.ForkCount,
			CreatedAt: fresh.CreatedAt, UpdatedAt: fresh.UpdatedAt, PushedAt: fresh.PushedAt,
		})
		if err != nil {
			continue
		}
		if err := db.UpsertDailyStar(d, id, today, fresh.StarCount); err != nil {
			continue
		}
		if prev.IsArchived != fresh.IsArchived {
			res.ArchivedFlipped++
		}
		res.Refreshed++
	}

	for _, id := range missing {
		if err := db.SoftDeleteByGitHubID(d, id, today); err != nil {
			continue
		}
		res.SoftDeleted++
	}

	res.FailureRate = float64(len(missing)) / float64(len(ids))

	if bulkErr != nil {
		return res, fmt.Errorf("bulk refresh: %w", bulkErr)
	}
	if res.FailureRate > maxFailureRate {
		return res, ErrFailureRateExceeded
	}
	return res, nil
}
