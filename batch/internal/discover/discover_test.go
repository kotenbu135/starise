package discover

import (
	"context"
	"errors"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

var errInjected = errors.New("injected search error")

func TestDiscoverInsertsNewRepos(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchResult = []github.RepoData{
		{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100},
		{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200},
	}

	res, err := Run(context.Background(), d, c, github.SearchOptions{Query: "stars:>10"}, "2026-04-18")
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 2 {
		t.Errorf("discovered=%d, want 2", res.Discovered)
	}
	if res.Refreshed != 0 {
		t.Errorf("refreshed=%d, want 0", res.Refreshed)
	}

	all, _ := db.ListActiveRepositories(d)
	if len(all) != 2 {
		t.Errorf("active=%d", len(all))
	}
}

func TestDiscoverRefreshesKnownRepos(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a"})

	c := github.NewMockClient()
	c.SearchResult = []github.RepoData{
		{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100},
		{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200},
	}

	res, _ := Run(context.Background(), d, c, github.SearchOptions{Query: "stars:>10"}, "2026-04-18")
	if res.Discovered != 1 {
		t.Errorf("discovered=%d", res.Discovered)
	}
	if res.Refreshed != 1 {
		t.Errorf("refreshed=%d", res.Refreshed)
	}
}

func TestDiscoverPersistsPartialOnSearchError(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchByQuery = map[string][]github.RepoData{
		"q": {
			{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100},
			{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200},
		},
	}
	c.SearchErr = map[string]error{"q": errInjected}

	res, err := Run(context.Background(), d, c, github.SearchOptions{Query: "q"}, "2026-04-18")
	if err == nil {
		t.Fatal("expected search error to surface")
	}
	if res.Discovered != 2 {
		t.Errorf("discovered=%d, want 2 (partial data must persist)", res.Discovered)
	}
	active, _ := db.ListActiveRepositories(d)
	if len(active) != 2 {
		t.Errorf("active=%d, want 2", len(active))
	}
}

func TestDiscoverRejectsEmptyQuery(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	if _, err := Run(context.Background(), d, c, github.SearchOptions{}, "2026-04-18"); err == nil {
		t.Errorf("expected error")
	}
}
