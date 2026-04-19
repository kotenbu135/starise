// Package discover finds new repositories via the Search API and writes
// today's snapshot for the ones we haven't seen before.
package discover

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

type Result struct {
	Discovered int // newly inserted
	Refreshed  int // already known but snapshot updated
	Errors     int
}

// Run runs the search and upserts each result. Today's snapshot is written
// for everything that came back. Existing repos are still updated so we
// always carry the latest metadata + today's stars.
func Run(ctx context.Context, d *sql.DB, c github.Client, opts github.SearchOptions, today string) (Result, error) {
	if opts.Query == "" {
		return Result{}, errors.New("discover: empty query")
	}
	// Partial-data contract: SearchRepos may return collected repos alongside
	// a pagination error (e.g. 1000-result cap on a late page). Persist what
	// came back before surfacing the error.
	results, _, searchErr := c.SearchRepos(ctx, opts)

	var res Result
	for _, r := range results {
		existing, err := db.GetRepositoryByGitHubID(d, r.GitHubID)
		isNew := err != nil

		id, err := db.UpsertRepository(d, db.Repository{
			GitHubID: r.GitHubID, Owner: r.Owner, Name: r.Name,
			Description: r.Description, URL: r.URL, HomepageURL: r.HomepageURL,
			Language: r.Language, License: r.License, Topics: r.Topics,
			IsArchived: r.IsArchived, IsFork: r.IsFork, ForkCount: r.ForkCount,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, PushedAt: r.PushedAt,
		})
		if err != nil {
			res.Errors++
			continue
		}
		if err := db.UpsertDailyStar(d, id, today, r.StarCount); err != nil {
			res.Errors++
			continue
		}
		if isNew || existing.GitHubID == "" {
			res.Discovered++
		} else {
			res.Refreshed++
		}
	}
	if searchErr != nil {
		return res, fmt.Errorf("search: %w", searchErr)
	}
	return res, nil
}
