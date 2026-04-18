package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
)

// I1: every non-deleted DB row has a matching repos/{owner}__{name}.json,
// AND rankings.json shows only active repos (non-archived, non-deleted).
func TestInvariantI1_Completeness_Real(t *testing.T) {
	d := openMem(t)
	mustUpsert(t, d, "G1", "x", "live", false, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	mustUpsert(t, d, "G2", "x", "arc", true, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	mustUpsert(t, d, "G3", "x", "del", false, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	db.SoftDeleteByGitHubID(d, "G3", "2026-04-18")

	dir := t.TempDir()
	if _, err := export.Export(d, export.Options{
		OutDir: dir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	// Lower bound: non-deleted rows have files.
	for _, name := range []string{"x__live.json", "x__arc.json"} {
		if _, err := os.Stat(filepath.Join(dir, "repos", name)); err != nil {
			t.Errorf("non-deleted JSON missing: %s", name)
		}
	}

	// rankings.json contains only active (live), not archived or deleted.
	b, err := os.ReadFile(filepath.Join(dir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rk export.Rankings
	if err := json.Unmarshal(b, &rk); err != nil {
		t.Fatal(err)
	}
	for slot, entries := range rk.Rankings {
		for _, e := range entries {
			if e.Owner != "x" || e.Name != "live" {
				t.Errorf("slot %s contains non-active repo: %s/%s", slot, e.Owner, e.Name)
			}
		}
	}
}
