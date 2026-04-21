package discover

import (
	"context"
	"errors"
	"fmt"
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

func TestRunManyCapturesQueryErrorSamples(t *testing.T) {
	// The 2026-04-20 CI run reported QueryErrors:23 with no detail. Without
	// the actual error messages we cannot tell whether they are rate-limit,
	// timeout, or query-syntax failures — each needs a different fix. This
	// test locks in the guarantee that the first N error messages are
	// captured verbatim so the next CI log will show WHY queries failed.
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchErr = map[string]error{
		"q1": errors.New("rate limited"),
		"q2": errors.New("search cap hit"),
		"q3": errors.New("timeout"),
	}

	res, err := RunMany(context.Background(), d, c,
		[]string{"ok1", "q1", "q2", "q3"}, "2026-04-18",
		RunManyOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.QueryErrors != 3 {
		t.Fatalf("queryErrors=%d, want 3", res.QueryErrors)
	}
	if len(res.QueryErrorSamples) != 3 {
		t.Fatalf("samples=%d, want 3 (all errors captured when under cap)", len(res.QueryErrorSamples))
	}
	// Each sample must include BOTH the query that failed AND the error msg
	// so operators can diagnose a failure class.
	seen := make(map[string]bool)
	for _, s := range res.QueryErrorSamples {
		seen[s] = true
	}
	for _, want := range []string{
		"q1: rate limited", "q2: search cap hit", "q3: timeout",
	} {
		if !seen[want] {
			t.Errorf("missing sample %q in %v", want, res.QueryErrorSamples)
		}
	}
}

func TestRunManyCapsQueryErrorSamplesAtTen(t *testing.T) {
	// Observability field should not grow unboundedly; cap at 10 samples so
	// a fully-broken run's log output stays readable. Count remains accurate.
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	errs := make(map[string]error, 20)
	queries := make([]string, 20)
	for i := 0; i < 20; i++ {
		q := fmt.Sprintf("q%02d", i)
		queries[i] = q
		errs[q] = fmt.Errorf("e%02d", i)
	}
	c.SearchErr = errs

	res, err := RunMany(context.Background(), d, c, queries, "2026-04-18",
		RunManyOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.QueryErrors != 20 {
		t.Errorf("queryErrors=%d, want 20", res.QueryErrors)
	}
	if len(res.QueryErrorSamples) != 10 {
		t.Errorf("samples=%d, want 10 (cap enforced)", len(res.QueryErrorSamples))
	}
}

func TestRunManyAggregatesRateLimitCostAndMinRemaining(t *testing.T) {
	// Empirical budget validation: sum of per-query Cost matches theoretical
	// calculation (or reveals the discrepancy), and min Remaining surfaces
	// whether any single query drove us close to the hourly budget edge.
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.SearchByQuery = map[string][]github.RepoData{
		"q1": {{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100}},
		"q2": {{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200}},
		"q3": {{GitHubID: "G3", Owner: "x", Name: "c", StarCount: 300}},
	}
	c.LimitByQuery = map[string]github.RateLimitInfo{
		"q1": {Cost: 20, Remaining: 4980},
		"q2": {Cost: 25, Remaining: 4955},
		"q3": {Cost: 22, Remaining: 4933},
	}

	res, err := RunMany(context.Background(), d, c,
		[]string{"q1", "q2", "q3"}, "2026-04-18",
		RunManyOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.CostTotal != 67 {
		t.Errorf("costTotal=%d, want 67 (sum of 20+25+22)", res.CostTotal)
	}
	if res.MinRemaining != 4933 {
		t.Errorf("minRemaining=%d, want 4933 (lowest observed)", res.MinRemaining)
	}
	if res.MaxCostPerQuery != 25 {
		t.Errorf("maxCostPerQuery=%d, want 25 (largest single query)", res.MaxCostPerQuery)
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
