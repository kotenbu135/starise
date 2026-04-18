package export

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func seedRepoWithStar(t *testing.T, d *sql.DB, owner, name, date string, stars int) {
	t.Helper()
	id, err := db.UpsertRepository(d, &db.Repository{
		GitHubID: owner + "/" + name, Owner: owner, Name: name,
		Topics: "[]", // required: export marshals this as raw JSON
	})
	if err != nil {
		t.Fatalf("upsert repo: %v", err)
	}
	if err := db.UpsertDailyStar(d, &db.DailyStar{RepoID: id, RecordedDate: date, StarCount: stars}); err != nil {
		t.Fatalf("upsert star: %v", err)
	}
}

func TestExport_RemovesOrphans(t *testing.T) {
	d := newTestDB(t)
	seedRepoWithStar(t, d, "acme", "alpha", "2026-04-17", 100)
	seedRepoWithStar(t, d, "acme", "beta", "2026-04-17", 200)

	outDir := t.TempDir()
	reposDir := filepath.Join(outDir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-seed stale files: one is expected to be cleaned, one will be overwritten
	orphan := filepath.Join(reposDir, "old-owner__dead-repo.json")
	if err := os.WriteFile(orphan, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stillHere := filepath.Join(reposDir, "acme__alpha.json")
	if err := os.WriteFile(stillHere, []byte(`{"stale": true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Export(d, outDir); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan still exists (err=%v)", err)
	}
	if _, err := os.Stat(stillHere); err != nil {
		t.Errorf("kept file missing: %v", err)
	}
	// acme__beta.json also created
	if _, err := os.Stat(filepath.Join(reposDir, "acme__beta.json")); err != nil {
		t.Errorf("expected beta.json: %v", err)
	}
}

func TestExport_EmptyRankingsAreJSONArrayNotNull(t *testing.T) {
	d := newTestDB(t)
	// no repos → rankings.json.rankings[period] should serialize as [], not null
	outDir := t.TempDir()
	if err := Export(d, outDir); err != nil {
		t.Fatalf("Export: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(outDir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f RankingsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, period := range []string{"1d", "7d", "30d"} {
		v, ok := f.Rankings[period]
		if !ok {
			t.Errorf("period %s missing", period)
			continue
		}
		if v == nil {
			t.Errorf("period %s is nil (should be empty slice)", period)
		}
		if len(v) != 0 {
			t.Errorf("period %s: len=%d, want 0", period, len(v))
		}
	}
	// Verify the wire format itself, not just the parsed struct: the raw JSON
	// must contain `[]`, not `null`, for each period.
	var wire struct {
		Rankings map[string]json.RawMessage `json:"rankings"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for period, v := range wire.Rankings {
		if string(v) == "null" {
			t.Errorf("period %s serialized as null, want []", period)
		}
	}
}

func TestExport_IgnoresNonJSONFilesInReposDir(t *testing.T) {
	d := newTestDB(t)
	seedRepoWithStar(t, d, "acme", "alpha", "2026-04-17", 100)

	outDir := t.TempDir()
	reposDir := filepath.Join(outDir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-.json files and subdirs must not be touched by orphan cleanup.
	if err := os.WriteFile(filepath.Join(reposDir, ".gitkeep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(reposDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Export(d, outDir); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, err := os.Stat(filepath.Join(reposDir, ".gitkeep")); err != nil {
		t.Errorf(".gitkeep removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(reposDir, "subdir")); err != nil {
		t.Errorf("subdir removed: %v", err)
	}
}
