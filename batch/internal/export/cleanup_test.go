package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
)

func TestCleanupRemovesOrphanFiles(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "alive"})

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "x__alive.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "x__ghost.json"), []byte("{}"), 0o644)

	res, err := Cleanup(d, dir, "2026-04-18")
	if err != nil {
		t.Fatal(err)
	}
	if res.OrphansRemoved != 1 {
		t.Errorf("orphans removed=%d", res.OrphansRemoved)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "x__alive.json")); err != nil {
		t.Errorf("alive removed by mistake")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "x__ghost.json")); !os.IsNotExist(err) {
		t.Errorf("ghost not removed")
	}
}

func TestCleanupHardDeletesPastWindow(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G_OLD", Owner: "x", Name: "old"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G_NEW", Owner: "x", Name: "new"})
	// "old" was soft-deleted 91 days before 2026-04-18 = 2026-01-17
	db.SoftDeleteByGitHubID(d, "G_OLD", "2026-01-17")
	// "new" was soft-deleted 30 days ago — still inside the 90-day window
	db.SoftDeleteByGitHubID(d, "G_NEW", "2026-03-19")

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "repos"), 0o755)
	os.WriteFile(filepath.Join(dir, "repos", "x__old.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "repos", "x__new.json"), []byte("{}"), 0o644)

	res, err := Cleanup(d, dir, "2026-04-18")
	if err != nil {
		t.Fatal(err)
	}
	if res.HardDeleted != 1 {
		t.Errorf("hard deleted=%d, want 1", res.HardDeleted)
	}
	if _, err := db.GetRepositoryByGitHubID(d, "G_OLD"); err == nil {
		t.Errorf("G_OLD still in DB")
	}
	if _, err := db.GetRepositoryByGitHubID(d, "G_NEW"); err != nil {
		t.Errorf("G_NEW removed prematurely: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "repos", "x__old.json")); !os.IsNotExist(err) {
		t.Errorf("old file not removed")
	}
}
