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

// TestRunAllWithRestoreRestoresHistoryBeforeFetch verifies the production
// GitHub Actions flow:
//
//  1. Fresh DB (no prior state).
//  2. restore reads data/repos/*.json and re-seeds daily_stars history.
//  3. fetch adds today's snapshot.
//  4. compute can now diff against yesterday → non-empty rankings.
//  5. export writes JSON.
//
// Without step 2, step 4 would produce empty rankings (no start snapshot).
func TestRunAllWithRestoreRestoresHistoryBeforeFetch(t *testing.T) {
	dataDir := t.TempDir()
	outDir := t.TempDir()

	// Pre-existing data/repos/acme__widget.json — simulates yesterday's run.
	reposDir := filepath.Join(dataDir, "repos")
	_ = os.MkdirAll(reposDir, 0o755)
	detail := export.RepoDetail{
		RepoID: "g", Owner: "acme", Name: "widget", FullName: "acme/widget",
		URL: "https://github.com/acme/widget", Topics: []string{},
		StarCount: 200,
		StarHistory: []export.StarPoint{
			{Date: "2026-04-11", Stars: 150},
			{Date: "2026-04-17", Stars: 200},
		},
	}
	b, _ := json.MarshalIndent(detail, "", "  ")
	_ = os.WriteFile(filepath.Join(reposDir, "acme__widget.json"), b, 0o644)

	// Fresh DB.
	d, _ := db.Open("")
	defer d.Close()

	// Mock supplies today's snapshot.
	mock := github.NewMock()
	mock.StubRepo("acme", "widget", github.RepoData{
		ID: "g", Owner: github.Owner{Login: "acme"}, Name: "widget",
		URL: "https://github.com/acme/widget", StargazerCount: 220,
	})

	opts := RunOptions{
		Client:       mock,
		RestoreFrom:  dataDir,
		Seeds:        []string{"acme/widget"},
		Today:        "2026-04-18",
		UpdatedAt:    "2026-04-18T00:00:00Z",
		ComputedDate: "2026-04-18",
		SkipDiscover: true,
		TopN:         50,
		OutDir:       outDir,
	}
	if err := RunAll(d, opts); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Rankings should exist for 7d (start 2026-04-11 @ 150, end 2026-04-18 @ 220).
	rows, err := db.ListRankings(d, "7d", "2026-04-18", 10)
	if err != nil {
		t.Fatalf("list rankings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("7d rows=%d want 1", len(rows))
	}
	// (220-150)/150*100 = 46.666...
	got := rows[0].GrowthPct
	if got < 46.66 || got > 46.67 {
		t.Errorf("7d growth: %v want ~46.67", got)
	}
}

func TestRunAllWithRestoreMissingDirReturnsError(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	opts := RunOptions{
		Client:       github.NewMock(),
		RestoreFrom:  "/nonexistent",
		SkipDiscover: true,
		Today:        "2026-04-18",
		UpdatedAt:    "2026-04-18T00:00:00Z",
		ComputedDate: "2026-04-18",
		OutDir:       t.TempDir(),
	}
	if err := RunAll(d, opts); err == nil {
		t.Error("expected error for missing restore dir")
	}
}
