package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// I13: re-running export against the same DB state with the same
// computed_date and the same UpdatedAt/GeneratedAt yields byte-identical
// rankings.json / meta.json / repos/*.json files.
func TestInvariantI13_DeterministicExport_Real(t *testing.T) {
	d := openMem(t)
	id1, _ := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a", Language: "Go",
		Topics: []string{"b", "a"},
	})
	db.UpsertDailyStar(d, id1, "2026-04-17", 5)
	db.UpsertDailyStar(d, id1, "2026-04-18", 100)
	id2, _ := db.UpsertRepository(d, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Language: "Rust",
	})
	db.UpsertDailyStar(d, id2, "2026-04-17", 100)
	db.UpsertDailyStar(d, id2, "2026-04-18", 250)

	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}

	dir1, dir2 := t.TempDir(), t.TempDir()
	opts := export.Options{UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100}
	opts.OutDir = dir1
	if _, err := export.Export(d, opts); err != nil {
		t.Fatal(err)
	}
	opts.OutDir = dir2
	if _, err := export.Export(d, opts); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"rankings.json", "meta.json", "repos/x__a.json", "repos/x__b.json"} {
		a, _ := os.ReadFile(filepath.Join(dir1, rel))
		b, _ := os.ReadFile(filepath.Join(dir2, rel))
		if string(a) != string(b) {
			t.Errorf("%s differs across runs", rel)
		}
	}
}
