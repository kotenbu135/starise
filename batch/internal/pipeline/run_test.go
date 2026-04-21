package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// partialBulkPipelineClient injects a BulkRefresh error while still returning
// partial data + aggregate rate-limit telemetry, so we can verify that the
// pipeline Report captures the refresh.Result even on error — without it,
// CI logs for a failed run show Refreshed:0 and no cost telemetry (which is
// exactly what the 2026-04-20 incident log showed).
type partialBulkPipelineClient struct {
	*github.MockClient
	found    []github.RepoData
	limit    github.RateLimitInfo
	bulkErr  error
}

func (p *partialBulkPipelineClient) BulkRefresh(_ context.Context, _ []string) ([]github.RepoData, []string, github.RateLimitInfo, error) {
	return p.found, nil, p.limit, p.bulkErr
}

func TestRunAllEndToEnd(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	c.Add(github.RepoData{GitHubID: "G1", Owner: "x", Name: "small", StarCount: 50})
	c.Add(github.RepoData{GitHubID: "G2", Owner: "x", Name: "big", StarCount: 1500})

	// Pre-seed yesterday's snapshot so the 1d window has real growth to rank.
	id1, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "small"})
	db.UpsertDailyStar(d, id1, "2026-04-17", 5)
	id2, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "x", Name: "big"})
	db.UpsertDailyStar(d, id2, "2026-04-17", 500)

	dir := t.TempDir()
	rep, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18",
		SeedOwners: []string{"x", "x"}, SeedNames: []string{"small", "big"},
		OutDir: dir, TopN: 100, SkipDiscover: true,
		UpdatedAt: "X", GeneratedAt: "X",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Fetched.Fetched != 2 {
		t.Errorf("fetched=%+v", rep.Fetched)
	}
	if rep.ExportRepos != 2 {
		t.Errorf("exported=%d", rep.ExportRepos)
	}
	if _, err := os.Stat(filepath.Join(dir, "rankings.json")); err != nil {
		t.Errorf("rankings.json missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err != nil {
		t.Errorf("meta.json missing")
	}
}

func TestRunAllReportsRefreshResultEvenOnTransientError(t *testing.T) {
	// Reproduces the 2026-04-20 incident: refresh returned EOF on batch 33
	// of ~290, aborted the pipeline, and the CI log showed Refreshed:0
	// despite 32 batches of successful partial data already persisted.
	// Fix: pipeline assigns report.Refreshed BEFORE surfacing the error so
	// operators can see where the run got to.
	d, _ := db.Open("")
	defer d.Close()

	// 3 repos in DB, 2 came back before the fault.
	for i, id := range []string{"G0", "G1", "G2"} {
		db.UpsertRepository(d, db.Repository{GitHubID: id, Owner: "x", Name: string(rune('a' + i))})
	}
	c := &partialBulkPipelineClient{
		MockClient: github.NewMockClient(),
		found: []github.RepoData{
			{GitHubID: "G0", Owner: "x", Name: "a", StarCount: 10},
			{GitHubID: "G1", Owner: "x", Name: "b", StarCount: 20},
		},
		limit: github.RateLimitInfo{
			Remaining: 2500, Cost: 160, MaxBatchCost: 21,
			ResetAt: "2026-04-20T17:00:00Z",
		},
		bulkErr: errors.New("batch 33 (100 ids): EOF"),
	}

	dir := t.TempDir()
	rep, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18", OutDir: dir, TopN: 100,
		SkipDiscover: true, UpdatedAt: "X", GeneratedAt: "X",
		AllowEmptyRankings: true,
	})
	if err == nil {
		t.Fatal("expected transient bulk error to surface")
	}
	// The telemetry MUST be visible in the report even though the run
	// aborted — this is the whole point of the fix.
	if rep.Refreshed.Refreshed != 2 {
		t.Errorf("rep.Refreshed.Refreshed=%d, want 2 (partial data preserved)",
			rep.Refreshed.Refreshed)
	}
	if rep.Refreshed.CostTotal != 160 {
		t.Errorf("rep.Refreshed.CostTotal=%d, want 160", rep.Refreshed.CostTotal)
	}
	if rep.Refreshed.MinRemaining != 2500 {
		t.Errorf("rep.Refreshed.MinRemaining=%d, want 2500", rep.Refreshed.MinRemaining)
	}
	if rep.Refreshed.MaxCostPerBatch != 21 {
		t.Errorf("rep.Refreshed.MaxCostPerBatch=%d, want 21", rep.Refreshed.MaxCostPerBatch)
	}
}

func TestRunAllRefreshFailureAborts(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	// Pre-populate DB with 10 repos
	c := github.NewMockClient()
	for i := 0; i < 10; i++ {
		ghID := string(rune('A' + i))
		db.UpsertRepository(d, db.Repository{GitHubID: ghID, Owner: "x", Name: ghID})
		if i < 5 { // 50% missing → above 30% threshold
			c.MissingIDs[ghID] = true
			continue
		}
		c.Add(github.RepoData{GitHubID: ghID, Owner: "x", Name: ghID, StarCount: 100})
	}

	dir := t.TempDir()
	_, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18", OutDir: dir, TopN: 100,
		SkipDiscover: true, UpdatedAt: "X", GeneratedAt: "X",
	})
	if err == nil {
		t.Errorf("expected refresh failure")
	}
}
