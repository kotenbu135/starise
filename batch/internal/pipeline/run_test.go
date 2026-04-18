package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/github"
	_ "modernc.org/sqlite"
)

// TestRunAllEndToEnd drives the full pipeline (fetch → compute → export)
// using a :memory: DB and a mocked GitHub client. This is the single
// integration test that proves the whole batch produces usable JSON.
func TestRunAllEndToEnd(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Pre-seed a 30-day-old snapshot directly (simulating yesterday's run).
	// This gives compute something to diff against when fetch adds today's.
	pre, _ := db.UpsertRepository(d, &db.Repository{
		GitHubID: "g-pre", Owner: "acme", Name: "widget", Language: "Go", Topics: "[]",
	})
	_ = db.UpsertDailyStar(d, &db.DailyStar{RepoID: pre, RecordedDate: "2026-03-19", StarCount: 100})
	_ = db.UpsertDailyStar(d, &db.DailyStar{RepoID: pre, RecordedDate: "2026-04-11", StarCount: 200})
	_ = db.UpsertDailyStar(d, &db.DailyStar{RepoID: pre, RecordedDate: "2026-04-17", StarCount: 280})

	mock := github.NewMock()
	mock.StubRepo("acme", "widget", github.RepoData{
		ID: "g-pre", Owner: github.Owner{Login: "acme"}, Name: "widget",
		URL: "https://github.com/acme/widget", StargazerCount: 300,
		PrimaryLang: &github.Language{Name: "Go"},
	})

	opts := RunOptions{
		Client:       mock,
		Seeds:        []string{"acme/widget"},
		Today:        "2026-04-18",
		UpdatedAt:    "2026-04-18T00:00:00Z",
		ComputedDate: "2026-04-18",
		SkipDiscover: true,
		TopN:         50,
		OutDir:       t.TempDir(),
	}
	if err := RunAll(d, opts); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Rankings written to DB for every period.
	for _, p := range []string{"1d", "7d", "30d"} {
		rows, _ := db.ListRankings(d, p, "2026-04-18", 10)
		if len(rows) != 1 {
			t.Errorf("period %s rows=%d want 1", p, len(rows))
		}
	}

	// JSON output files exist and are well-formed.
	b, err := os.ReadFile(filepath.Join(opts.OutDir, "rankings.json"))
	if err != nil {
		t.Fatalf("rankings.json: %v", err)
	}
	var r export.Rankings
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("parse rankings: %v", err)
	}
	if len(r.Rankings["7d"]) != 1 {
		t.Errorf("7d entries: %d", len(r.Rankings["7d"]))
	}
	// 7d back from 2026-04-18 = 2026-04-11 → start 200, end 300 → 50%
	if r.Rankings["7d"][0].GrowthPct != 50.0 {
		t.Errorf("7d pct: %v want 50.0", r.Rankings["7d"][0].GrowthPct)
	}

	// Repo detail exists.
	detailPath := filepath.Join(opts.OutDir, "repos", "acme__widget.json")
	if _, err := os.Stat(detailPath); err != nil {
		t.Errorf("repo detail missing: %v", err)
	}

	// meta.json has total_repos==1.
	var meta export.Meta
	mb, _ := os.ReadFile(filepath.Join(opts.OutDir, "meta.json"))
	_ = json.Unmarshal(mb, &meta)
	if meta.TotalRepos != 1 {
		t.Errorf("total_repos: %d", meta.TotalRepos)
	}
}
