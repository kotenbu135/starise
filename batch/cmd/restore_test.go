package cmd

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"

	_ "modernc.org/sqlite"
)

func newRestoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func writeRepoJSON(t *testing.T, dir, owner, name string, history []map[string]any) {
	t.Helper()
	payload := map[string]any{
		"owner":       owner,
		"name":        name,
		"description": "desc",
		"url":         "https://github.com/" + owner + "/" + name,
		"language":    "Go",
		"license":     "MIT",
		"topics":      []string{"test"},
		"fork_count":  0,
		"star_count":  0,
		"is_archived": false,
	}
	if history != nil {
		payload["star_history"] = history
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, owner+"__"+name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func countDailyStarsForRepo(t *testing.T, d *sql.DB, owner, name string) int {
	t.Helper()
	var n int
	err := d.QueryRow(`
		SELECT COUNT(*) FROM daily_stars ds JOIN repositories r ON r.id = ds.repo_id
		WHERE r.owner = ? AND r.name = ?`, owner, name).Scan(&n)
	if err != nil {
		t.Fatalf("count stars: %v", err)
	}
	return n
}

func TestRestore_PopulatesReposAndHistory(t *testing.T) {
	tmp := t.TempDir()
	reposDir := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoJSON(t, reposDir, "acme", "alpha", []map[string]any{
		{"date": "2026-04-15", "stars": 90},
		{"date": "2026-04-16", "stars": 95},
		{"date": "2026-04-17", "stars": 100},
	})
	writeRepoJSON(t, reposDir, "acme", "beta", []map[string]any{
		{"date": "2026-04-17", "stars": 50},
	})

	d := newRestoreTestDB(t)
	if err := Restore(d, tmp); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	var repos int
	if err := d.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&repos); err != nil {
		t.Fatal(err)
	}
	if repos != 2 {
		t.Errorf("repos = %d, want 2", repos)
	}
	if n := countDailyStarsForRepo(t, d, "acme", "alpha"); n != 3 {
		t.Errorf("alpha history = %d, want 3", n)
	}
	if n := countDailyStarsForRepo(t, d, "acme", "beta"); n != 1 {
		t.Errorf("beta history = %d, want 1", n)
	}
}

func TestRestore_MissingDirSucceedsEmpty(t *testing.T) {
	d := newRestoreTestDB(t)
	// no repos/ dir inside empty tmp
	if err := Restore(d, t.TempDir()); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	var n int
	d.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&n)
	if n != 0 {
		t.Errorf("repos = %d, want 0", n)
	}
}

func TestRestore_SkipsMalformedFiles(t *testing.T) {
	tmp := t.TempDir()
	reposDir := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid file
	writeRepoJSON(t, reposDir, "acme", "good", []map[string]any{{"date": "2026-04-17", "stars": 10}})
	// Malformed JSON — must be skipped, not halt restore
	if err := os.WriteFile(filepath.Join(reposDir, "bad__broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Missing owner/name — must be skipped
	if err := os.WriteFile(filepath.Join(reposDir, "empty__fields.json"), []byte(`{"owner":""}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := newRestoreTestDB(t)
	if err := Restore(d, tmp); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	var n int
	d.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&n)
	if n != 1 {
		t.Errorf("repos = %d, want 1 (only 'good' should survive)", n)
	}
}

func TestRestore_IsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	reposDir := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoJSON(t, reposDir, "acme", "alpha", []map[string]any{
		{"date": "2026-04-17", "stars": 100},
	})

	d := newRestoreTestDB(t)
	for i := 0; i < 3; i++ {
		if err := Restore(d, tmp); err != nil {
			t.Fatalf("Restore iter %d: %v", i, err)
		}
	}
	var repos, stars int
	d.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&repos)
	d.QueryRow("SELECT COUNT(*) FROM daily_stars").Scan(&stars)
	if repos != 1 || stars != 1 {
		t.Errorf("after 3x Restore: repos=%d stars=%d, want 1/1 (UPSERT must dedupe)", repos, stars)
	}
}

func TestRestore_PreservesTopicsAsJSONArray(t *testing.T) {
	tmp := t.TempDir()
	reposDir := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoJSON(t, reposDir, "acme", "topical", []map[string]any{{"date": "2026-04-17", "stars": 1}})

	d := newRestoreTestDB(t)
	if err := Restore(d, tmp); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	var topics string
	err := d.QueryRow("SELECT topics FROM repositories WHERE owner='acme' AND name='topical'").Scan(&topics)
	if err != nil {
		t.Fatal(err)
	}
	// Must be valid JSON array (required by export's json.RawMessage marshaling).
	var parsed []string
	if err := json.Unmarshal([]byte(topics), &parsed); err != nil {
		t.Errorf("topics not valid JSON array: %q (%v)", topics, err)
	}
}
