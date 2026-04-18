package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/kotenbu135/starise/batch/internal/restore"
)

// I11: data/ is the source of truth. After wiping the DB, restoring from
// data/, and re-running compute + export, the JSON files must be byte-equal
// to the original (modulo updated_at/generated_at).
func TestInvariantI11_DataIsSourceOfTruth_Real(t *testing.T) {
	src := openMem(t)
	id1, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a", Language: "Go",
		Topics: []string{"a", "b"},
	})
	db.UpsertDailyStar(src, id1, "2026-04-17", 5)
	db.UpsertDailyStar(src, id1, "2026-04-18", 100)
	id2, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Language: "Rust",
	})
	db.UpsertDailyStar(src, id2, "2026-04-17", 100)
	db.UpsertDailyStar(src, id2, "2026-04-18", 250)

	if err := ranking.Compute(src, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}
	originalDir := t.TempDir()
	if _, err := export.Export(src, export.Options{
		OutDir: originalDir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	// Wipe DB, restore from data/, re-compute, re-export.
	dst := openMem(t)
	if _, err := restore.FromDir(dst, originalDir); err != nil {
		t.Fatal(err)
	}
	if err := ranking.Compute(dst, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}
	rebuiltDir := t.TempDir()
	if _, err := export.Export(dst, export.Options{
		OutDir: rebuiltDir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	// Compare byte-by-byte.
	for _, rel := range []string{"rankings.json", "meta.json", "repos/x__a.json", "repos/x__b.json"} {
		a, err := os.ReadFile(filepath.Join(originalDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(rebuiltDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs after restore+re-export", rel)
		}
	}
}
