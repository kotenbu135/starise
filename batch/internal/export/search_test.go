package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// BuildSearchIndex must list non-archived, non-deleted repos sorted by
// (owner, name) so re-runs produce byte-identical output (I13-compatible).
func TestBuildSearchIndex_Sorted(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "zeta", Name: "b"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "alpha", Name: "a"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G3", Owner: "alpha", Name: "z"})

	idx, err := BuildSearchIndex(d, "2026-04-30T00:00:00Z", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Repos) != 3 {
		t.Fatalf("len=%d, want 3", len(idx.Repos))
	}
	want := []string{"alpha/a", "alpha/z", "zeta/b"}
	for i, e := range idx.Repos {
		if e.O+"/"+e.N != want[i] {
			t.Errorf("idx[%d]=%s/%s, want %s", i, e.O, e.N, want[i])
		}
	}
}

// description_ja takes precedence over description; fallback to description
// when the translation cache misses or returns empty.
func TestBuildSearchIndex_DescriptionFallback(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "translated", Description: "original english"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "x", Name: "untranslated", Description: "english only"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G3", Owner: "x", Name: "empty", Description: ""})

	cacheDir := t.TempDir()
	c := &translate.Cache{Dir: cacheDir}
	if err := c.Put(translate.CacheEntry{
		Src: "original english", JA: "原文の日本語", Provider: "mock",
		TranslatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	idx, err := BuildSearchIndex(d, "t", "2026-04-30", c)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range idx.Repos {
		got[e.N] = e.D
	}
	if got["translated"] != "原文の日本語" {
		t.Errorf("translated.D=%q, want 原文の日本語", got["translated"])
	}
	if got["untranslated"] != "english only" {
		t.Errorf("untranslated.D=%q, want fallback to english only", got["untranslated"])
	}
	if got["empty"] != "" {
		t.Errorf("empty.D=%q, want empty", got["empty"])
	}
}

// Description must be truncated to 80 runes (not bytes) so multi-byte
// Japanese characters are not split mid-codepoint.
func TestBuildSearchIndex_TruncatesUTF8(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	long := ""
	for i := 0; i < 200; i++ {
		long += "あ"
	}
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "long", Description: long})

	idx, err := BuildSearchIndex(d, "t", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Repos) != 1 {
		t.Fatalf("len=%d, want 1", len(idx.Repos))
	}
	got := []rune(idx.Repos[0].D)
	if len(got) != 80 {
		t.Errorf("rune len=%d, want 80", len(got))
	}
	for _, r := range got {
		if r != 'あ' {
			t.Errorf("unexpected rune %q (utf8 split?)", r)
			break
		}
	}
}

// soft-deleted and archived repos must not appear in the index.
func TestBuildSearchIndex_ExcludesDeletedAndArchived(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "live"})
	db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "x", Name: "archived", IsArchived: true})
	db.UpsertRepository(d, db.Repository{GitHubID: "G3", Owner: "x", Name: "deleted"})
	db.SoftDeleteByGitHubID(d, "G3", "2026-04-29")

	idx, err := BuildSearchIndex(d, "t", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Repos) != 1 {
		t.Fatalf("len=%d, want 1 (only live)", len(idx.Repos))
	}
	if idx.Repos[0].N != "live" {
		t.Errorf("got name=%q, want live", idx.Repos[0].N)
	}
}

// Two BuildSearchIndex calls with the same DB + same generatedAt must
// produce byte-identical JSON when marshalled.
func TestBuildSearchIndex_Deterministic(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	id1, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a", Description: "first"})
	id2, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G2", Owner: "x", Name: "b", Description: "second"})
	db.UpsertDailyStar(d, id1, "2026-04-30", 42)
	db.UpsertDailyStar(d, id2, "2026-04-30", 99)

	idx1, err := BuildSearchIndex(d, "fixed", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := BuildSearchIndex(d, "fixed", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(idx1)
	b, _ := json.Marshal(idx2)
	if string(a) != string(b) {
		t.Errorf("not deterministic:\nA=%s\nB=%s", a, b)
	}
}

// star count is sourced from the most recent daily_stars snapshot ≤ computedDate.
func TestBuildSearchIndex_StarCountFromHistory(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	id, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a"})
	db.UpsertDailyStar(d, id, "2026-04-28", 100)
	db.UpsertDailyStar(d, id, "2026-04-30", 250)
	// snapshot after computedDate must NOT leak into result.
	db.UpsertDailyStar(d, id, "2026-05-01", 9999)

	idx, err := BuildSearchIndex(d, "t", "2026-04-30", nil)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Repos[0].S != 250 {
		t.Errorf("star count=%d, want 250", idx.Repos[0].S)
	}
}

// Export() integration: search-index.json file is produced alongside
// rankings/repos/meta and shaped correctly.
func TestExportWritesSearchIndex(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a", Language: "Go", Description: "hello"})

	dir := t.TempDir()
	if _, err := Export(d, Options{
		OutDir: dir, UpdatedAt: "now", GeneratedAt: "now", ComputedDate: "2026-04-30", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "search-index.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read search-index.json: %v", err)
	}
	var idx SearchIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatal(err)
	}
	if len(idx.Repos) != 1 {
		t.Fatalf("len=%d, want 1", len(idx.Repos))
	}
	e := idx.Repos[0]
	if e.O != "x" || e.N != "a" || e.L != "Go" || e.D != "hello" {
		t.Errorf("entry=%+v, want {O:x N:a L:Go D:hello}", e)
	}
}
