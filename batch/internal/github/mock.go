package github

import (
	"fmt"
	"sync"
)

// Mock is an in-memory Client suitable for unit and integration tests.
// Tests stub the responses they expect via StubRepo / StubSearch. Any
// un-stubbed FetchRepo call returns ErrNotFound.
type Mock struct {
	mu sync.Mutex

	repos   map[string]RepoData
	search  map[string][]RepoData
	fetchN  map[string]int
	searchN map[string]int
}

// NewMock returns an empty Mock.
func NewMock() *Mock {
	return &Mock{
		repos:   make(map[string]RepoData),
		search:  make(map[string][]RepoData),
		fetchN:  make(map[string]int),
		searchN: make(map[string]int),
	}
}

// StubRepo registers a canned response for FetchRepo(owner, name).
func (m *Mock) StubRepo(owner, name string, r RepoData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repos[repoKey(owner, name)] = r
}

// StubSearch registers a canned page for SearchRepos(query, ...).
// The returned page has HasNext=false; pagination is not simulated.
func (m *Mock) StubSearch(query string, repos []RepoData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.search[query] = repos
}

// FetchRepo implements Client.
func (m *Mock) FetchRepo(owner, name string) (*FetchRepoResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := repoKey(owner, name)
	m.fetchN[key]++
	r, ok := m.repos[key]
	if !ok {
		return nil, fmt.Errorf("mock: %s/%s: %w", owner, name, ErrNotFound)
	}
	return &FetchRepoResult{
		Repo:      r,
		RateLimit: RateLimitInfo{Limit: 5000, Remaining: 5000, Cost: 1},
	}, nil
}

// SearchRepos implements Client.
func (m *Mock) SearchRepos(query string, first int, after string) (*SearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchN[query]++
	repos, ok := m.search[query]
	if !ok {
		return &SearchResult{}, nil
	}
	return &SearchResult{
		Total:     len(repos),
		Repos:     repos,
		HasNext:   false,
		EndCursor: "",
		RateLimit: RateLimitInfo{Limit: 5000, Remaining: 5000, Cost: 1},
	}, nil
}

// FetchRepoCalls returns how many times FetchRepo(owner, name) was called.
func (m *Mock) FetchRepoCalls(owner, name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fetchN[repoKey(owner, name)]
}

// SearchReposCalls returns how many times SearchRepos(query, ...) was called.
func (m *Mock) SearchReposCalls(query string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.searchN[query]
}

func repoKey(owner, name string) string {
	return owner + "/" + name
}
