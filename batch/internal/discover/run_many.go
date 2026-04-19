package discover

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	"golang.org/x/sync/errgroup"
)

// ManyResult is returned by RunMany — aggregate counts across every query.
type ManyResult struct {
	QueriesRun  int
	QueryErrors int // SearchRepos failures per query
	Discovered  int // unique newly-inserted repos
	Refreshed   int // unique already-known repos touched
	Errors      int // DB upsert failures
}

// RunManyOptions controls the multi-query discover run.
type RunManyOptions struct {
	// Concurrency caps parallel Search API calls. 0 or negative defaults to 1.
	Concurrency int
	// MaxPages is forwarded to SearchOptions.MaxPages. 0 lets the client
	// choose its own default (currently 10).
	MaxPages int
	// PerPage is forwarded to SearchOptions.PerPage. 0 lets the client
	// choose its own default (currently 50 in the single-query path; the
	// GraphQL Search API allows up to 100).
	PerPage int
}

// RunMany dispatches each query in parallel, capped at opts.Concurrency, and
// upserts every resulting repo into d. Duplicate repos across queries are
// deduplicated in-process so the DB sees each repo at most once per run.
//
// A single query failing (rate limit, transient network error) is counted in
// QueryErrors but does not abort the batch — other queries still run.
func RunMany(ctx context.Context, d *sql.DB, c github.Client, queries []string, today string, opts RunManyOptions) (ManyResult, error) {
	if len(queries) == 0 {
		return ManyResult{}, errors.New("discover: empty query list")
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	type fetched struct {
		repos []github.RepoData
		err   error
	}
	out := make([]fetched, len(queries))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for i, q := range queries {
		if q == "" {
			continue
		}
		i, q := i, q
		g.Go(func() error {
			repos, _, err := c.SearchRepos(gctx, github.SearchOptions{
				Query:    q,
				MaxPages: opts.MaxPages,
				PerPage:  opts.PerPage,
			})
			out[i] = fetched{repos: repos, err: err}
			// Swallow per-query errors so one rate-limit hit on a band does
			// not cancel sibling queries via errgroup. QueryErrors surfaces
			// the count for observability.
			return nil
		})
	}
	// g.Wait always returns nil here since every goroutine returns nil, but
	// we still check defensively in case future changes propagate errors.
	if err := g.Wait(); err != nil {
		return ManyResult{}, err
	}

	res := ManyResult{QueriesRun: len(queries)}
	seen := make(map[string]bool)

	// DB writes run here on the main goroutine, after all fetchers have
	// finished. No mutex needed — single writer.
	//
	// Partial-data contract: SearchRepos may return both repos and an error
	// (e.g. hit the Search API 1000-result cap on page 10 after collecting
	// 999 repos on pages 1..9). We persist whatever came back AND count the
	// query as failed, so a single bad page never loses the other 999.
	for _, f := range out {
		if f.err != nil {
			res.QueryErrors++
		}
		for _, r := range f.repos {
			if seen[r.GitHubID] {
				continue
			}
			seen[r.GitHubID] = true

			existing, gerr := db.GetRepositoryByGitHubID(d, r.GitHubID)
			isNew := gerr != nil
			id, uerr := db.UpsertRepository(d, db.Repository{
				GitHubID: r.GitHubID, Owner: r.Owner, Name: r.Name,
				Description: r.Description, URL: r.URL, HomepageURL: r.HomepageURL,
				Language: r.Language, License: r.License, Topics: r.Topics,
				IsArchived: r.IsArchived, IsFork: r.IsFork, ForkCount: r.ForkCount,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, PushedAt: r.PushedAt,
			})
			if uerr != nil {
				res.Errors++
				continue
			}
			if serr := db.UpsertDailyStar(d, id, today, r.StarCount); serr != nil {
				res.Errors++
				continue
			}
			if isNew || existing.GitHubID == "" {
				res.Discovered++
			} else {
				res.Refreshed++
			}
		}
	}
	return res, nil
}
