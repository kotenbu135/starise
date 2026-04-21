package discover

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

	// Observability — populated for post-run diagnosis, not used by the
	// pipeline. Samples cap at queryErrorSampleCap so a fully-broken run
	// doesn't blow up logs. CostTotal/MinRemaining/MaxCostPerQuery let us
	// compare the actual GraphQL budget consumption to theoretical
	// calculations without instrumenting every request site.
	QueryErrorSamples []string
	CostTotal         int
	MinRemaining      int
	MaxCostPerQuery   int
}

// queryErrorSampleCap limits how many per-query error messages we keep in
// ManyResult.QueryErrorSamples. Ten is enough to see the failure class
// without drowning the run summary log when every query is broken.
const queryErrorSampleCap = 10

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
		limit github.RateLimitInfo
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
			repos, limit, err := c.SearchRepos(gctx, github.SearchOptions{
				Query:    q,
				MaxPages: opts.MaxPages,
				PerPage:  opts.PerPage,
			})
			out[i] = fetched{repos: repos, limit: limit, err: err}
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
	haveLimit := false

	// DB writes run here on the main goroutine, after all fetchers have
	// finished. No mutex needed — single writer.
	//
	// Partial-data contract: SearchRepos may return both repos and an error
	// (e.g. hit the Search API 1000-result cap on page 10 after collecting
	// 999 repos on pages 1..9). We persist whatever came back AND count the
	// query as failed, so a single bad page never loses the other 999.
	for i, f := range out {
		if f.err != nil {
			res.QueryErrors++
			if len(res.QueryErrorSamples) < queryErrorSampleCap {
				// Pair query with error so the operator can tell which
				// queries failed (not just how many).
				res.QueryErrorSamples = append(res.QueryErrorSamples,
					fmt.Sprintf("%s: %s", queries[i], f.err.Error()))
			}
		}
		// Aggregate rate-limit telemetry regardless of success: even a
		// failed query may return partial limit data (e.g. timeout after
		// 3 pages succeeded), and we want those points reflected.
		if f.limit != (github.RateLimitInfo{}) {
			res.CostTotal += f.limit.Cost
			if f.limit.Cost > res.MaxCostPerQuery {
				res.MaxCostPerQuery = f.limit.Cost
			}
			if !haveLimit || f.limit.Remaining < res.MinRemaining {
				res.MinRemaining = f.limit.Remaining
				haveLimit = true
			}
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
