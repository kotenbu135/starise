package github

import (
	"context"
	"errors"
	"testing"
)

func TestMockFetchRepo(t *testing.T) {
	m := NewMockClient()
	m.Add(RepoData{GitHubID: "G1", Owner: "ACME", Name: "Widget", StarCount: 100})

	r, _, err := m.FetchRepo(context.Background(), "acme", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if r.Owner != "acme" || r.Name != "widget" {
		t.Errorf("normalize failed: %+v", r)
	}
	if r.StarCount != 100 {
		t.Errorf("stars: %d", r.StarCount)
	}
}

func TestMockFetchRepoCaseInsensitive(t *testing.T) {
	m := NewMockClient()
	m.Add(RepoData{GitHubID: "G1", Owner: "acme", Name: "widget"})
	if _, _, err := m.FetchRepo(context.Background(), "ACME", "WIDGET"); err != nil {
		t.Errorf("case lookup: %v", err)
	}
}

func TestMockFetchRepoNotFound(t *testing.T) {
	m := NewMockClient()
	_, _, err := m.FetchRepo(context.Background(), "ghost", "town")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestMockBulkRefresh(t *testing.T) {
	m := NewMockClient()
	m.Add(RepoData{GitHubID: "A", Owner: "x", Name: "a"})
	m.Add(RepoData{GitHubID: "B", Owner: "x", Name: "b"})
	m.MissingIDs["C"] = true

	found, missing, _, err := m.BulkRefresh(context.Background(), []string{"A", "B", "C", "D"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Errorf("found len=%d, want 2", len(found))
	}
	if len(missing) != 2 || missing[0] != "C" || missing[1] != "D" {
		t.Errorf("missing=%v", missing)
	}
}
