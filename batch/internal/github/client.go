// Package github abstracts GraphQL access to the GitHub API. The Client
// interface is the only seam used by fetch/discover/refresh, so production
// code can be swapped with the in-memory MockClient in tests.
package github

import (
	"context"
	"errors"
	"strings"
)

// RepoData is the normalized snapshot returned by GraphQL queries.
// Owner and Name are lowercase per project convention.
type RepoData struct {
	GitHubID    string
	Owner       string
	Name        string
	Description string
	URL         string
	HomepageURL string
	Language    string
	License     string
	Topics      []string
	StarCount   int
	ForkCount   int
	IsArchived  bool
	IsFork      bool
	CreatedAt   string
	UpdatedAt   string
	PushedAt    string
}

// SearchOptions controls the discover query.
type SearchOptions struct {
	Query    string
	MaxPages int
	PerPage  int
}

// RateLimitInfo is returned with each call so callers can throttle.
type RateLimitInfo struct {
	Remaining int
	Cost      int
	ResetAt   string // RFC3339, empty when unknown
}

// ErrNotFound is returned by FetchRepo / BulkRefresh when a repo no longer
// exists. Callers translate this into a soft delete.
var ErrNotFound = errors.New("github: repository not found")

// Client is the GraphQL surface used by the batch processor.
type Client interface {
	// FetchRepo retrieves a single repo plus its current star count.
	// Returns ErrNotFound when the repo has been deleted.
	FetchRepo(ctx context.Context, owner, name string) (RepoData, RateLimitInfo, error)

	// SearchRepos paginates through Search API results.
	SearchRepos(ctx context.Context, opts SearchOptions) ([]RepoData, RateLimitInfo, error)

	// BulkRefresh retrieves the current star count for many repos in a
	// single GraphQL call (`nodes(ids: [...])`). Returns the data for repos
	// that still exist, the IDs that came back missing (404 / NOT_FOUND),
	// and the rate limit snapshot.
	BulkRefresh(ctx context.Context, githubIDs []string) ([]RepoData, []string, RateLimitInfo, error)
}

// Normalize lowercases owner/name fields. Use at every entry point so the
// DB unique key is stable regardless of GitHub's casing.
func Normalize(r RepoData) RepoData {
	r.Owner = strings.ToLower(r.Owner)
	r.Name = strings.ToLower(r.Name)
	return r
}
