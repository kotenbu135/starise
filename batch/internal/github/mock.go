package github

import (
	"context"
	"sort"
	"strings"
)

// MockClient is an in-memory Client used by tests. Repos are keyed by
// "owner/name" (lowercase). MissingIDs simulates 404 responses from
// BulkRefresh, and FetchOwners simulates 404 from FetchRepo.
type MockClient struct {
	Repos        map[string]RepoData // key = "owner/name"
	MissingNames map[string]bool     // key = "owner/name"
	MissingIDs   map[string]bool     // GitHub node ID -> 404
	SearchResult []RepoData
	Limit        RateLimitInfo
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
	m.Repos[r.Owner+"/"+r.Name] = r
}

func (m *MockClient) FetchRepo(_ context.Context, owner, name string) (RepoData, RateLimitInfo, error) {
	key := strings.ToLower(owner) + "/" + strings.ToLower(name)
	if m.MissingNames[key] {
		return RepoData{}, m.Limit, ErrNotFound
	}
	r, ok := m.Repos[key]
	if !ok {
		return RepoData{}, m.Limit, ErrNotFound
	}
	return r, m.Limit, nil
}

func (m *MockClient) SearchRepos(_ context.Context, _ SearchOptions) ([]RepoData, RateLimitInfo, error) {
	out := make([]RepoData, len(m.SearchResult))
	for i, r := range m.SearchResult {
		out[i] = Normalize(r)
	}
	return out, m.Limit, nil
}

func (m *MockClient) BulkRefresh(_ context.Context, ids []string) ([]RepoData, []string, RateLimitInfo, error) {
	// Build an index by GitHubID for the lookup.
	byID := map[string]RepoData{}
	for _, r := range m.Repos {
		byID[r.GitHubID] = r
	}
	var found []RepoData
	var missing []string
	for _, id := range ids {
		if m.MissingIDs[id] {
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
	return found, missing, m.Limit, nil
}
