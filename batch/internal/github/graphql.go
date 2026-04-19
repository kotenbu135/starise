package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shurcooL/graphql"
)

// GraphQLClient is the production GitHub GraphQL implementation.
type GraphQLClient struct {
	c *graphql.Client
}

// NewGraphQLClient builds a client using a GitHub personal access token.
// Requests are wrapped in retryTransport so GitHub secondary rate limits
// (403 with "rate limit" body) and 429 responses are retried — see
// retry.go. maxRetries=4 covers bursts without hiding sustained outages.
func NewGraphQLClient(token string) *GraphQLClient {
	httpClient := &http.Client{
		Transport: newRetryTransport(&authTransport{token: token}, 4),
		Timeout:   60 * time.Second,
	}
	return &GraphQLClient{c: graphql.NewClient("https://api.github.com/graphql", httpClient)}
}

type authTransport struct{ token string }

func (a *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer "+a.token)
	r.Header.Set("Accept", "application/vnd.github.v4+json")
	return http.DefaultTransport.RoundTrip(r)
}

type repoFragment struct {
	ID            graphql.String
	NameWithOwner graphql.String
	Owner         struct{ Login graphql.String }
	Name          graphql.String
	Description   graphql.String
	URL           graphql.String `graphql:"url"`
	HomepageURL   graphql.String `graphql:"homepageUrl"`
	IsArchived    graphql.Boolean
	IsFork        graphql.Boolean
	CreatedAt     graphql.String
	UpdatedAt     graphql.String
	PushedAt      graphql.String
	StargazerCount graphql.Int
	ForkCount     graphql.Int
	PrimaryLanguage struct {
		Name graphql.String
	}
	LicenseInfo struct {
		SPDXID graphql.String `graphql:"spdxId"`
	}
	RepositoryTopics struct {
		Nodes []struct {
			Topic struct{ Name graphql.String }
		}
	} `graphql:"repositoryTopics(first: 20)"`
}

type rateLimitFragment struct {
	Cost      graphql.Int
	Remaining graphql.Int
	ResetAt   graphql.String
}

func toRepoData(r repoFragment) RepoData {
	topics := make([]string, 0, len(r.RepositoryTopics.Nodes))
	for _, n := range r.RepositoryTopics.Nodes {
		topics = append(topics, string(n.Topic.Name))
	}
	d := RepoData{
		GitHubID:    string(r.ID),
		Owner:       string(r.Owner.Login),
		Name:        string(r.Name),
		Description: string(r.Description),
		URL:         string(r.URL),
		HomepageURL: string(r.HomepageURL),
		Language:    string(r.PrimaryLanguage.Name),
		License:     string(r.LicenseInfo.SPDXID),
		Topics:      topics,
		StarCount:   int(r.StargazerCount),
		ForkCount:   int(r.ForkCount),
		IsArchived:  bool(r.IsArchived),
		IsFork:      bool(r.IsFork),
		CreatedAt:   string(r.CreatedAt),
		UpdatedAt:   string(r.UpdatedAt),
		PushedAt:    string(r.PushedAt),
	}
	return Normalize(d)
}

func toRateLimit(r rateLimitFragment) RateLimitInfo {
	return RateLimitInfo{
		Remaining: int(r.Remaining),
		Cost:      int(r.Cost),
		ResetAt:   string(r.ResetAt),
	}
}

func (g *GraphQLClient) FetchRepo(ctx context.Context, owner, name string) (RepoData, RateLimitInfo, error) {
	var q struct {
		Repository repoFragment `graphql:"repository(owner: $owner, name: $name)"`
		RateLimit  rateLimitFragment
	}
	vars := map[string]interface{}{
		"owner": graphql.String(owner),
		"name":  graphql.String(name),
	}
	if err := g.c.Query(ctx, &q, vars); err != nil {
		if isNotFound(err) {
			return RepoData{}, toRateLimit(q.RateLimit), ErrNotFound
		}
		return RepoData{}, RateLimitInfo{}, fmt.Errorf("graphql FetchRepo: %w", err)
	}
	return toRepoData(q.Repository), toRateLimit(q.RateLimit), nil
}

func (g *GraphQLClient) SearchRepos(ctx context.Context, opts SearchOptions) ([]RepoData, RateLimitInfo, error) {
	if opts.PerPage <= 0 {
		opts.PerPage = 50
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 10
	}

	var out []RepoData
	var lastLimit RateLimitInfo
	var cursor *graphql.String

	for page := 0; page < opts.MaxPages; page++ {
		var q struct {
			Search struct {
				PageInfo struct {
					EndCursor   graphql.String
					HasNextPage graphql.Boolean
				}
				Edges []struct {
					Node struct {
						Repository repoFragment `graphql:"... on Repository"`
					}
				}
			} `graphql:"search(query: $query, type: REPOSITORY, first: $first, after: $cursor)"`
			RateLimit rateLimitFragment
		}
		vars := map[string]interface{}{
			"query":  graphql.String(opts.Query),
			"first":  graphql.Int(opts.PerPage),
			"cursor": cursor,
		}
		if err := g.c.Query(ctx, &q, vars); err != nil {
			return out, lastLimit, fmt.Errorf("graphql SearchRepos page %d: %w", page, err)
		}
		lastLimit = toRateLimit(q.RateLimit)
		for _, e := range q.Search.Edges {
			out = append(out, toRepoData(e.Node.Repository))
		}
		if !bool(q.Search.PageInfo.HasNextPage) {
			break
		}
		c := q.Search.PageInfo.EndCursor
		cursor = &c
	}
	return out, lastLimit, nil
}

const (
	bulkBatchSize   = 100
	bulkConcurrency = 5
)

// batchFetcher fetches a single GraphQL nodes() batch. Production uses
// GraphQLClient.fetchBatch; tests inject fakes to exercise the parallel
// aggregation without hitting the network.
type batchFetcher func(ctx context.Context, ids []string) ([]RepoData, []string, RateLimitInfo, error)

// runBulkRefreshParallel splits ids into batchSize chunks and runs at most
// concurrency fetchers in parallel. Aggregation is order-preserving so the
// concatenated output matches a sequential run. The returned RateLimitInfo
// is the most conservative snapshot observed across all shards (lowest
// Remaining) so callers make throttling decisions against the worst-case
// state, not whichever shard happened to land at a given index.
//
// Partial-data contract: if any batch errors, the function returns the
// first error encountered AND the data from every batch that did succeed.
// Callers (refresh.Run) persist the partial data before propagating the
// error so one rate-limit hit mid-run doesn't lose the other 29900 repos.
func runBulkRefreshParallel(ctx context.Context, ids []string, batchSize, concurrency int, fetch batchFetcher) ([]RepoData, []string, RateLimitInfo, error) {
	if len(ids) == 0 {
		return nil, nil, RateLimitInfo{}, nil
	}
	if batchSize <= 0 {
		batchSize = bulkBatchSize
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	type shard struct {
		found   []RepoData
		missing []string
		limit   RateLimitInfo
		err     error
	}
	var batches [][]string
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batches = append(batches, ids[start:end])
	}
	shards := make([]shard, len(batches))

	// We intentionally use a plain sync.WaitGroup + semaphore rather than
	// errgroup.WithContext because we do NOT want a single batch failure
	// to cancel sibling batches — each batch must get a fair chance to
	// complete and contribute its data.
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, batch := range batches {
		i, batch := i, batch
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			found, missing, limit, err := fetch(ctx, batch)
			if err != nil {
				shards[i] = shard{err: fmt.Errorf("batch %d (%d ids): %w", i, len(batch), err)}
				return
			}
			shards[i] = shard{found: found, missing: missing, limit: limit}
		}()
	}
	wg.Wait()

	var allFound []RepoData
	var allMissing []string
	var firstErr error
	// Pick the most conservative rate-limit snapshot (lowest Remaining) so
	// downstream throttling decisions use the worst observed state across
	// all shards, not whichever shard happened to land at a given index.
	var minLimit RateLimitInfo
	haveLimit := false
	for _, s := range shards {
		if s.err != nil {
			if firstErr == nil {
				firstErr = s.err
			}
			continue
		}
		allFound = append(allFound, s.found...)
		allMissing = append(allMissing, s.missing...)
		empty := s.limit == (RateLimitInfo{})
		if empty {
			continue
		}
		if !haveLimit || s.limit.Remaining < minLimit.Remaining {
			minLimit = s.limit
			haveLimit = true
		}
	}
	return allFound, allMissing, minLimit, firstErr
}

// fetchBatch issues a single nodes() GraphQL call for one id batch.
// Each invocation allocates its own query struct so the shurcooL/graphql
// client is safe for concurrent use under errgroup.
func (g *GraphQLClient) fetchBatch(ctx context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
	gqlIDs := make([]graphql.ID, len(batch))
	for i, id := range batch {
		gqlIDs[i] = graphql.ID(id)
	}
	var q struct {
		Nodes []struct {
			Repository repoFragment `graphql:"... on Repository"`
		} `graphql:"nodes(ids: $ids)"`
		RateLimit rateLimitFragment
	}
	vars := map[string]interface{}{"ids": gqlIDs}
	if err := g.c.Query(ctx, &q, vars); err != nil {
		return nil, nil, RateLimitInfo{}, err
	}
	var found []RepoData
	var missing []string
	for i, n := range q.Nodes {
		if n.Repository.ID == "" {
			missing = append(missing, batch[i])
			continue
		}
		found = append(found, toRepoData(n.Repository))
	}
	return found, missing, toRateLimit(q.RateLimit), nil
}

func (g *GraphQLClient) BulkRefresh(ctx context.Context, ids []string) ([]RepoData, []string, RateLimitInfo, error) {
	return runBulkRefreshParallel(ctx, ids, bulkBatchSize, bulkConcurrency, g.fetchBatch)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Could not resolve to a Repository") ||
		strings.Contains(msg, "NOT_FOUND")
}

// SleepUntilReset blocks the caller until rate limit resets, plus a 5s buffer,
// when remaining drops below the given threshold.
func SleepUntilReset(info RateLimitInfo, threshold int, now time.Time) time.Duration {
	if info.Remaining >= threshold || info.ResetAt == "" {
		return 0
	}
	reset, err := time.Parse(time.RFC3339, info.ResetAt)
	if err != nil {
		return 0
	}
	if !reset.After(now) {
		return 0
	}
	return reset.Sub(now) + 5*time.Second
}

// Sentinel for callers wanting to detect rate-limit failures explicitly.
var ErrRateLimited = errors.New("github: rate limited")
