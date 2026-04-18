package github

import (
	"errors"
	"testing"
)

func TestMockClientFetchRepoReturnsStubbed(t *testing.T) {
	mock := NewMock()
	mock.StubRepo("acme", "widget", RepoData{
		ID:             "gid1",
		Owner:          Owner{Login: "acme"},
		Name:           "widget",
		StargazerCount: 123,
	})

	result, err := mock.FetchRepo("acme", "widget")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if result.Repo.StargazerCount != 123 {
		t.Errorf("stars: %d", result.Repo.StargazerCount)
	}
}

func TestMockClientFetchRepoUnknownReturnsError(t *testing.T) {
	mock := NewMock()
	_, err := mock.FetchRepo("missing", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockClientSearchReposReturnsStubbed(t *testing.T) {
	mock := NewMock()
	mock.StubSearch("stars:>100", []RepoData{
		{ID: "a", Owner: Owner{Login: "o1"}, Name: "r1", StargazerCount: 150},
		{ID: "b", Owner: Owner{Login: "o2"}, Name: "r2", StargazerCount: 200},
	})

	res, err := mock.SearchRepos("stars:>100", 100, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Repos) != 2 {
		t.Errorf("len=%d", len(res.Repos))
	}
	if res.HasNext {
		t.Errorf("mock should mark last page as HasNext=false")
	}
}

func TestMockClientRecordsCallCounts(t *testing.T) {
	mock := NewMock()
	mock.StubRepo("a", "b", RepoData{ID: "x", Owner: Owner{Login: "a"}, Name: "b"})
	_, _ = mock.FetchRepo("a", "b")
	_, _ = mock.FetchRepo("a", "b")
	if n := mock.FetchRepoCalls("a", "b"); n != 2 {
		t.Errorf("expected 2 calls, got %d", n)
	}
}
