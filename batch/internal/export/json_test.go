package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	_ "modernc.org/sqlite"
)

func TestExportWritesAllFiles(t *testing.T) {
	d, err := db.Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed two repos + stars + rankings.
	a, _ := db.UpsertRepository(d, &db.Repository{
		GitHubID: "gA", Owner: "acme", Name: "widget", Language: "Go",
		Description: "first", URL: "https://github.com/acme/widget",
		Topics: `["ai","cli"]`,
	})
	b, _ := db.UpsertRepository(d, &db.Repository{
		GitHubID: "gB", Owner: "foo", Name: "bar", Language: "Rust",
		URL: "https://github.com/foo/bar", Topics: "[]",
	})
	for _, s := range []db.DailyStar{
		{RepoID: a, RecordedDate: "2026-04-11", StarCount: 100},
		{RepoID: a, RecordedDate: "2026-04-18", StarCount: 150},
		{RepoID: b, RecordedDate: "2026-04-11", StarCount: 200},
		{RepoID: b, RecordedDate: "2026-04-18", StarCount: 220},
	} {
		_ = db.UpsertDailyStar(d, &s)
	}
	for _, period := range []string{"1d", "7d", "30d"} {
		_ = db.ReplaceRankingsForDate(d, period, "2026-04-18", []db.Ranking{
			{RepoID: a, Period: period, ComputedDate: "2026-04-18",
				StartStars: 100, EndStars: 150, StarDelta: 50, GrowthPct: 50.0, Rank: 1},
			{RepoID: b, Period: period, ComputedDate: "2026-04-18",
				StartStars: 200, EndStars: 220, StarDelta: 20, GrowthPct: 10.0, Rank: 2},
		})
	}

	dir := t.TempDir()
	if err := Export(d, dir, "2026-04-18T00:00:00Z", "2026-04-18", 10); err != nil {
		t.Fatalf("export: %v", err)
	}

	// rankings.json
	var r Rankings
	mustReadJSON(t, filepath.Join(dir, "rankings.json"), &r)
	if len(r.Rankings["7d"]) != 2 {
		t.Errorf("rankings.json 7d entries=%d", len(r.Rankings["7d"]))
	}
	if r.Rankings["7d"][0].FullName != "acme/widget" {
		t.Errorf("top full_name: %q", r.Rankings["7d"][0].FullName)
	}
	if r.Rankings["7d"][0].Language != "Go" {
		t.Errorf("language not populated: %q", r.Rankings["7d"][0].Language)
	}

	// meta.json
	var meta Meta
	mustReadJSON(t, filepath.Join(dir, "meta.json"), &meta)
	if meta.TotalRepos != 2 {
		t.Errorf("meta total_repos: %d", meta.TotalRepos)
	}
	if len(meta.Periods) != 3 {
		t.Errorf("meta periods: %v", meta.Periods)
	}

	// repo detail
	var detail RepoDetail
	mustReadJSON(t, filepath.Join(dir, "repos", "acme__widget.json"), &detail)
	if detail.FullName != "acme/widget" || detail.StarCount != 150 {
		t.Errorf("detail mismatch: %+v", detail)
	}
	if len(detail.StarHistory) != 2 {
		t.Errorf("history len: %d", len(detail.StarHistory))
	}
	if detail.StarHistory[0].Date != "2026-04-11" || detail.StarHistory[1].Date != "2026-04-18" {
		t.Errorf("history order: %+v", detail.StarHistory)
	}
	if detail.Topics[0] != "ai" {
		t.Errorf("topics: %v", detail.Topics)
	}
}

func TestExportDetailIsDeterministic(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)

	_, _ = db.UpsertRepository(d, &db.Repository{GitHubID: "g", Owner: "o", Name: "r", Topics: "[]"})
	_ = db.UpsertDailyStar(d, &db.DailyStar{RepoID: 1, RecordedDate: "2026-04-18", StarCount: 1})
	_ = db.ReplaceRankingsForDate(d, "7d", "2026-04-18", []db.Ranking{
		{RepoID: 1, Period: "7d", ComputedDate: "2026-04-18", StartStars: 10, EndStars: 12, StarDelta: 2, GrowthPct: 20.0, Rank: 1},
	})

	dir := t.TempDir()
	if err := Export(d, dir, "2026-04-18T00:00:00Z", "2026-04-18", 10); err != nil {
		t.Fatalf("export: %v", err)
	}

	b1, err := os.ReadFile(filepath.Join(dir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Run export a second time into a fresh dir.
	dir2 := t.TempDir()
	if err := Export(d, dir2, "2026-04-18T00:00:00Z", "2026-04-18", 10); err != nil {
		t.Fatalf("second export: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(dir2, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Errorf("non-deterministic rankings.json")
	}
}

func mustReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
