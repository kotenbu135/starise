package restore

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
)

func TestFromDirRestoresReposAndHistory(t *testing.T) {
	src, _ := db.Open("")
	defer src.Close()
	id, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a", Language: "Go",
		Topics: []string{"ai", "cli"}, IsArchived: false,
	})
	db.UpsertDailyStar(src, id, "2026-04-17", 50)
	db.UpsertDailyStar(src, id, "2026-04-18", 100)

	id2, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Language: "Rust",
	})
	db.UpsertDailyStar(src, id2, "2026-04-18", 500)
	_ = id2

	dir := t.TempDir()
	if _, err := export.Export(src, export.Options{
		OutDir: dir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	dst, _ := db.Open("")
	defer dst.Close()
	res, err := FromDir(dst, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Repos != 2 || res.Snapshots != 3 {
		t.Errorf("res=%+v", res)
	}

	// Verify metadata round-trip
	got, err := db.GetRepositoryByGitHubID(dst, "G1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != "x" || got.Name != "a" || got.Language != "Go" {
		t.Errorf("metadata mismatch: %+v", got)
	}
	hist, _ := db.ListStarHistory(dst, got.ID)
	if len(hist) != 2 {
		t.Errorf("history len=%d", len(hist))
	}
}

func TestFromDirPreservesSoftDelete(t *testing.T) {
	src, _ := db.Open("")
	defer src.Close()
	db.UpsertRepository(src, db.Repository{GitHubID: "G1", Owner: "x", Name: "del"})
	db.SoftDeleteByGitHubID(src, "G1", "2026-04-18")

	dir := t.TempDir()
	export.Export(src, export.Options{OutDir: dir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100})

	dst, _ := db.Open("")
	defer dst.Close()
	if _, err := FromDir(dst, dir); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetRepositoryByGitHubID(dst, "G1")
	if got.DeletedAt != "2026-04-18" {
		t.Errorf("deleted_at = %q", got.DeletedAt)
	}
}
