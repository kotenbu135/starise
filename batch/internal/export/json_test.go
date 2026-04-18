package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// ---- I8: rankings.json contains all 6 keys (even when empty) ----
func TestExportEmptyDBStillEmitsSixKeys(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	dir := t.TempDir()

	if _, err := Export(d, Options{
		OutDir: dir, UpdatedAt: "2026-04-18T15:00:00Z",
		GeneratedAt: "2026-04-18T15:00:00Z", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rk Rankings
	if err := json.Unmarshal(b, &rk); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for k := range rk.Rankings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := AllRankingKeys()
	sort.Strings(want)
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("got %v, want %v", keys, want)
	}
}

// I1 (lower bound) + I2 (round-trip, deleted_at preserved): export writes
// JSON for every row in the DB — non-deleted, archived, AND soft-deleted —
// so that restore can rebuild the DB with identical deleted_at values.
// Hard-deleted rows are gone from the DB and skipped by export; their
// orphan JSON gets cleaned up on the next Cleanup() pass.
func TestExportWritesOneFilePerRepo(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "live"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "x", Name: "arc", IsArchived: true})
	db.UpsertRepository(d, db.Repository{GitHubID: "G3", Owner: "x", Name: "del"})
	db.SoftDeleteByGitHubID(d, "G3", "2026-04-18")

	dir := t.TempDir()
	written, err := Export(d, Options{
		OutDir: dir, UpdatedAt: "now", GeneratedAt: "now", ComputedDate: "2026-04-18", TopN: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if written != 3 {
		t.Errorf("written=%d, want 3 (live + arc + del)", written)
	}
	files, _ := os.ReadDir(filepath.Join(dir, "repos"))
	names := []string{}
	for _, f := range files {
		names = append(names, f.Name())
	}
	sort.Strings(names)
	want := []string{"x__arc.json", "x__del.json", "x__live.json"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

// ---- I13: re-export with same DB state and same options is byte-identical ----
func TestExportIsDeterministic(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	id, _ := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a", Language: "Go",
		Topics: []string{"b", "a", "c"},
	})
	db.UpsertDailyStar(d, id, "2026-04-17", 5)
	db.UpsertDailyStar(d, id, "2026-04-18", 100)
	id2, _ := db.UpsertRepository(d, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Language: "Rust",
	})
	db.UpsertDailyStar(d, id2, "2026-04-17", 100)
	db.UpsertDailyStar(d, id2, "2026-04-18", 200)
	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}

	dir1, dir2 := t.TempDir(), t.TempDir()
	opts := Options{UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100}
	opts.OutDir = dir1
	if _, err := Export(d, opts); err != nil {
		t.Fatal(err)
	}
	opts.OutDir = dir2
	if _, err := Export(d, opts); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"rankings.json", "meta.json", "repos/x__a.json", "repos/x__b.json"} {
		a, err := os.ReadFile(filepath.Join(dir1, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		b, err := os.ReadFile(filepath.Join(dir2, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs between runs", rel)
		}
	}
}

