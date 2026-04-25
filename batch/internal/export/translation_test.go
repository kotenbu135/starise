package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// description_ja must be populated when the cache has a hit.
func TestExport_DescriptionJA_PopulatedFromCache(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	desc := "A blazing-fast HTTP router."
	if _, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "router", Description: desc,
	}); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")
	c := &translate.Cache{Dir: cacheDir}
	if err := c.Put(translate.CacheEntry{
		Src: desc, JA: "高速な HTTP ルーター。", Provider: "claude", TranslatedAt: "2026-04-26T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := Export(d, Options{
		OutDir:              dataDir,
		UpdatedAt:           "now", GeneratedAt: "now", ComputedDate: "2026-04-18", TopN: 100,
		TranslationCacheDir: cacheDir,
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "repos", "x__router.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got RepoDetail
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DescriptionJA != "高速な HTTP ルーター。" {
		t.Fatalf("DescriptionJA = %q, want translated", got.DescriptionJA)
	}
	if got.Description != desc {
		t.Fatalf("Description must remain English original, got %q", got.Description)
	}
}

// description_ja stays empty on cache miss; original description is unaffected.
func TestExport_DescriptionJA_EmptyOnCacheMiss(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	if _, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "miss", Description: "untranslated text",
	}); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Export(d, Options{
		OutDir:              dataDir,
		UpdatedAt:           "now", GeneratedAt: "now", ComputedDate: "2026-04-18", TopN: 100,
		TranslationCacheDir: cacheDir,
	}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, "repos", "x__miss.json"))
	var got RepoDetail
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DescriptionJA != "" {
		t.Fatalf("DescriptionJA should be empty on miss, got %q", got.DescriptionJA)
	}
	if got.Description != "untranslated text" {
		t.Fatalf("Description corrupted: %q", got.Description)
	}
}

// When TranslationCacheDir is empty, no cache lookups are attempted and
// description_ja is left blank — preserving backwards compatibility for
// callers (and tests) that don't know about translation yet.
func TestExport_DescriptionJA_BlankWhenNoCacheDir(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	if _, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "y", Description: "anything",
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if _, err := Export(d, Options{
		OutDir: dir, UpdatedAt: "now", GeneratedAt: "now",
		ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "repos", "x__y.json"))
	var got RepoDetail
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DescriptionJA != "" {
		t.Fatalf("DescriptionJA must be empty without cache dir, got %q", got.DescriptionJA)
	}
}

// Determinism (I13) must hold even with the cache wired up: same DB +
// same cache + same options → byte-identical output.
func TestExport_DescriptionJA_DeterministicWithCache(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	if _, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a", Description: "thing",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Description: "other",
	}); err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	c := &translate.Cache{Dir: cacheDir}
	_ = c.Put(translate.CacheEntry{Src: "thing", JA: "もの", Provider: "claude", TranslatedAt: "t"})
	_ = c.Put(translate.CacheEntry{Src: "other", JA: "他", Provider: "claude", TranslatedAt: "t"})

	dir1, dir2 := t.TempDir(), t.TempDir()
	opts := Options{
		UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
		TranslationCacheDir: cacheDir,
	}
	opts.OutDir = dir1
	if _, err := Export(d, opts); err != nil {
		t.Fatal(err)
	}
	opts.OutDir = dir2
	if _, err := Export(d, opts); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"repos/x__a.json", "repos/x__b.json"} {
		a, _ := os.ReadFile(filepath.Join(dir1, rel))
		b, _ := os.ReadFile(filepath.Join(dir2, rel))
		if string(a) != string(b) {
			t.Errorf("%s differs between runs", rel)
		}
	}
}
