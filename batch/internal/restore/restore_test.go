package restore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	_ "modernc.org/sqlite"
)

// writeDetail marshals a RepoDetail to {owner}__{name}.json under dir/repos/.
func writeDetail(t *testing.T, dir string, detail export.RepoDetail) {
	t.Helper()
	reposDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(reposDir, detail.Owner+"__"+detail.Name+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestFromDirPopulatesRepositoriesAndStars(t *testing.T) {
	dir := t.TempDir()
	writeDetail(t, dir, export.RepoDetail{
		RepoID: "g1", Owner: "acme", Name: "widget", FullName: "acme/widget",
		Description: "test", URL: "https://github.com/acme/widget",
		Language: "Go", License: "MIT", Topics: []string{"ai", "cli"},
		StarCount: 150, ForkCount: 20,
		StarHistory: []export.StarPoint{
			{Date: "2026-04-11", Stars: 100},
			{Date: "2026-04-18", Stars: 150},
		},
	})
	writeDetail(t, dir, export.RepoDetail{
		RepoID: "g2", Owner: "foo", Name: "bar", FullName: "foo/bar",
		URL: "https://github.com/foo/bar", Topics: []string{},
		StarCount: 50,
		StarHistory: []export.StarPoint{
			{Date: "2026-04-18", Stars: 50},
		},
	})

	d, _ := db.Open("")
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	stats, err := FromDir(d, dir)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if stats.Repos != 2 {
		t.Errorf("repos=%d want 2", stats.Repos)
	}
	if stats.StarPoints != 3 {
		t.Errorf("star_points=%d want 3", stats.StarPoints)
	}

	// Repo records reconstructed.
	r, err := db.GetRepositoryByOwnerName(d, "acme", "widget")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if r.Language != "Go" || r.License != "MIT" {
		t.Errorf("metadata not restored: %+v", r)
	}

	// Topics round-trip through JSON string.
	var topics []string
	if err := json.Unmarshal([]byte(r.Topics), &topics); err != nil {
		t.Fatalf("topics json: %v", err)
	}
	if len(topics) != 2 || topics[0] != "ai" {
		t.Errorf("topics lost: %v", topics)
	}

	// daily_stars reconstructed for both dates.
	s, err := db.GetDailyStar(d, r.ID, "2026-04-11")
	if err != nil {
		t.Fatalf("star 04-11: %v", err)
	}
	if s.StarCount != 100 {
		t.Errorf("star 04-11: %d want 100", s.StarCount)
	}
}

func TestFromDirIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeDetail(t, dir, export.RepoDetail{
		RepoID: "g", Owner: "o", Name: "r", FullName: "o/r",
		Topics: []string{}, StarCount: 10,
		StarHistory: []export.StarPoint{{Date: "2026-04-18", Stars: 10}},
	})

	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)

	if _, err := FromDir(d, dir); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := FromDir(d, dir); err != nil {
		t.Fatalf("second: %v", err)
	}

	list, _ := db.ListRepositories(d)
	if len(list) != 1 {
		t.Errorf("duplicate repo rows on re-restore: %d", len(list))
	}
	snaps, _ := db.ListDailyStars(d, list[0].ID)
	if len(snaps) != 1 {
		t.Errorf("duplicate stars on re-restore: %d", len(snaps))
	}
}

func TestFromDirMissingDirReturnsError(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)
	if _, err := FromDir(d, "/nonexistent/path"); err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestFromDirNoReposSubdirReturnsEmptyStats(t *testing.T) {
	dir := t.TempDir() // exists but has no repos/ subdir
	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)

	stats, err := FromDir(d, dir)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if stats.Repos != 0 {
		t.Errorf("expected 0 repos, got %d", stats.Repos)
	}
}

func TestFromDirSkipsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	_ = os.MkdirAll(reposDir, 0o755)

	// Valid file.
	writeDetail(t, dir, export.RepoDetail{
		RepoID: "g", Owner: "o", Name: "ok", FullName: "o/ok",
		Topics: []string{}, StarCount: 10,
		StarHistory: []export.StarPoint{{Date: "2026-04-18", Stars: 10}},
	})
	// Malformed file.
	if err := os.WriteFile(filepath.Join(reposDir, "bad__file.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-json file (should be ignored).
	if err := os.WriteFile(filepath.Join(reposDir, "README.md"), []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)

	stats, err := FromDir(d, dir)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if stats.Repos != 1 {
		t.Errorf("repos=%d want 1", stats.Repos)
	}
	if stats.Failed != 1 {
		t.Errorf("failed=%d want 1", stats.Failed)
	}
}

func TestFromDirSkipsEntryMissingOwnerOrName(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	_ = os.MkdirAll(reposDir, 0o755)

	// RepoDetail with blank owner.
	b, _ := json.Marshal(export.RepoDetail{
		RepoID: "g", Name: "x", FullName: "/x",
		Topics: []string{}, StarCount: 1,
		StarHistory: []export.StarPoint{{Date: "2026-04-18", Stars: 1}},
	})
	_ = os.WriteFile(filepath.Join(reposDir, "blank.json"), b, 0o644)

	d, _ := db.Open("")
	defer d.Close()
	_ = db.Migrate(d)

	stats, err := FromDir(d, dir)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if stats.Repos != 0 {
		t.Errorf("expected 0 repos restored, got %d", stats.Repos)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failed)
	}
}
