package discover

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

func TestRunManyExecutesAllQueries(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchByQuery = map[string][]github.RepoData{
		"stars:>=50000": {{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 60000}},
		"stars:>=10000": {{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 20000}},
		"topic:llm":     {{GitHubID: "G3", Owner: "x", Name: "c", StarCount: 500}},
	}

	queries := []string{"stars:>=50000", "stars:>=10000", "topic:llm"}
	res, err := RunMany(context.Background(), d, c, queries, "2026-04-18", RunManyOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 3 {
		t.Errorf("discovered=%d, want 3", res.Discovered)
	}
	if res.QueriesRun != 3 {
		t.Errorf("queriesRun=%d, want 3", res.QueriesRun)
	}

	// Every query must have been executed.
	sort.Strings(c.SearchCalls)
	sort.Strings(queries)
	if len(c.SearchCalls) != len(queries) {
		t.Fatalf("calls=%d, want %d", len(c.SearchCalls), len(queries))
	}
	for i := range queries {
		if c.SearchCalls[i] != queries[i] {
			t.Errorf("call[%d]=%q, want %q", i, c.SearchCalls[i], queries[i])
		}
	}
}

func TestRunManyDeduplicatesOverlappingResults(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	// Same repo appears in two queries — must not be double-counted.
	shared := github.RepoData{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100}
	c.SearchByQuery = map[string][]github.RepoData{
		"q1": {shared},
		"q2": {shared, {GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200}},
	}

	res, err := RunMany(context.Background(), d, c, []string{"q1", "q2"}, "2026-04-18", RunManyOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 2 {
		t.Errorf("discovered=%d, want 2 (dedup)", res.Discovered)
	}
	active, _ := db.ListActiveRepositories(d)
	if len(active) != 2 {
		t.Errorf("active=%d, want 2", len(active))
	}
}

func TestRunManyContinuesOnSingleQueryError(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchByQuery = map[string][]github.RepoData{
		"ok": {{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100}},
	}
	c.SearchErr = map[string]error{
		"bad": errors.New("rate limited"),
	}

	res, err := RunMany(context.Background(), d, c, []string{"ok", "bad"}, "2026-04-18", RunManyOptions{Concurrency: 2})
	if err != nil {
		t.Fatalf("one-query failure must not abort: %v", err)
	}
	if res.Discovered != 1 {
		t.Errorf("discovered=%d, want 1", res.Discovered)
	}
	if res.QueryErrors != 1 {
		t.Errorf("queryErrors=%d, want 1", res.QueryErrors)
	}
}

func TestRunManyRejectsEmptyQueries(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	if _, err := RunMany(context.Background(), d, c, nil, "2026-04-18", RunManyOptions{Concurrency: 2}); err == nil {
		t.Error("expected error for empty queries")
	}
}

func TestRunManyConcurrencyDefaultsToOne(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchResult = []github.RepoData{{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100}}

	res, err := RunMany(context.Background(), d, c, []string{"q1"}, "2026-04-18", RunManyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 1 {
		t.Errorf("discovered=%d, want 1", res.Discovered)
	}
}

func TestRunManyPreservesPartialDataOnQueryError(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	// The "partial" query yields 2 repos alongside an error — simulates
	// the Search API returning 999 results before hitting its 1000-cap
	// on page 10. The repos that did come back must not be lost.
	c.SearchByQuery = map[string][]github.RepoData{
		"partial": {
			{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100},
			{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200},
		},
		"ok": {{GitHubID: "G3", Owner: "x", Name: "c", StarCount: 300}},
	}
	c.SearchErr = map[string]error{
		"partial": errors.New("page 10: Search API 1000-result cap"),
	}

	res, err := RunMany(context.Background(), d, c, []string{"partial", "ok"}, "2026-04-18",
		RunManyOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 3 {
		t.Errorf("discovered=%d, want 3 (all repos persisted despite partial failure)", res.Discovered)
	}
	if res.QueryErrors != 1 {
		t.Errorf("queryErrors=%d, want 1", res.QueryErrors)
	}
	active, _ := db.ListActiveRepositories(d)
	if len(active) != 3 {
		t.Errorf("active=%d, want 3", len(active))
	}
}

func TestRunManyPropagatesMaxPagesAndPerPage(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchResult = []github.RepoData{{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100}}

	_, err := RunMany(context.Background(), d, c, []string{"q1", "q2"}, "2026-04-18",
		RunManyOptions{Concurrency: 2, MaxPages: 7, PerPage: 75})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SearchOpts) != 2 {
		t.Fatalf("SearchOpts=%d, want 2", len(c.SearchOpts))
	}
	for i, opts := range c.SearchOpts {
		if opts.MaxPages != 7 {
			t.Errorf("call[%d].MaxPages=%d, want 7", i, opts.MaxPages)
		}
		if opts.PerPage != 75 {
			t.Errorf("call[%d].PerPage=%d, want 75", i, opts.PerPage)
		}
	}
}
