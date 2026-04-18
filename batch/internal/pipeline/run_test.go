package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

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
