package github

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// MockClient is an in-memory Client used by tests. Repos are keyed by
// "owner/name" (lowercase). MissingIDs simulates 404 responses from
// BulkRefresh, and FetchOwners simulates 404 from FetchRepo.
type MockClient struct {
	Repos        map[string]RepoData // key = "owner/name"
	MissingNames map[string]bool     // key = "owner/name"
	MissingIDs   map[string]bool     // GitHub node ID -> 404
	SearchResult []RepoData
	// SearchByQuery lets tests return different fixtures per Search query.
	// When set, it takes precedence over SearchResult; unknown queries fall
	// back to SearchResult.
	SearchByQuery map[string][]RepoData
	// SearchCalls records the queries each SearchRepos call saw, in order.
	// Tests use this to verify multi-query dispatch.
	SearchCalls []string
	// SearchOpts records the full SearchOptions each call observed, in order.
	// Tests use this to verify MaxPages / PerPage propagation.
	SearchOpts []SearchOptions
	// SearchErr forces SearchRepos to return this error for matching queries.
	// When the matching query also has an entry in SearchByQuery, the repos
	// are returned alongside the error so callers can exercise the
	// "partial data + error" contract (e.g. Search API 1000-result cap).
	SearchErr map[string]error
	mu        sync.Mutex
	Limit     RateLimitInfo
}

func NewMockClient() *MockClient {
	return &MockClient{
		Repos:        map[string]RepoData{},
		MissingNames: map[string]bool{},
		MissingIDs:   map[string]bool{},
	}
}

func (m *MockClient) Add(r RepoData) {
	r = Normalize(r)
	m.mu.Lock()
	m.Repos[r.Owner+"/"+r.Name] = r
	m.mu.Unlock()
}

func (m *MockClient) FetchRepo(_ context.Context, owner, name string) (RepoData, RateLimitInfo, error) {
	key := strings.ToLower(owner) + "/" + strings.ToLower(name)
	m.mu.Lock()
	missing := m.MissingNames[key]
	r, ok := m.Repos[key]
	limit := m.Limit
	m.mu.Unlock()
	if missing || !ok {
		return RepoData{}, limit, ErrNotFound
	}
	return r, limit, nil
}

func (m *MockClient) SearchRepos(_ context.Context, opts SearchOptions) ([]RepoData, RateLimitInfo, error) {
	m.mu.Lock()
	m.SearchCalls = append(m.SearchCalls, opts.Query)
	m.SearchOpts = append(m.SearchOpts, opts)
	src, hasData := m.SearchByQuery[opts.Query]
	if !hasData {
		src = m.SearchResult
	}
	out := make([]RepoData, len(src))
	for i, r := range src {
		out[i] = Normalize(r)
	}
	err := m.SearchErr[opts.Query]
	limit := m.Limit
	m.mu.Unlock()
	return out, limit, err
}

func (m *MockClient) BulkRefresh(_ context.Context, ids []string) ([]RepoData, []string, RateLimitInfo, error) {
	m.mu.Lock()
	// Build an index by GitHubID for the lookup. Guarded by mu so parallel
	// BulkRefresh + Add / SearchRepos calls are race-free under -race.
	byID := make(map[string]RepoData, len(m.Repos))
	for _, r := range m.Repos {
		byID[r.GitHubID] = r
	}
	missingSet := make(map[string]bool, len(m.MissingIDs))
	for id := range m.MissingIDs {
		missingSet[id] = true
	}
	limit := m.Limit
	m.mu.Unlock()

	var found []RepoData
	var missing []string
	for _, id := range ids {
		if missingSet[id] {
			missing = append(missing, id)
			continue
		}
		r, ok := byID[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		found = append(found, r)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].GitHubID < found[j].GitHubID })
	sort.Strings(missing)
	return found, missing, limit, nil
}
