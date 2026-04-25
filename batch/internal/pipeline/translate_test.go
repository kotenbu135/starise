package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// When a Translator + cache dir are configured, RunAll translates each
// repo's description, writes the cache, and Export injects description_ja.
func TestRunAll_TranslatesAndInjectsDescriptionJA(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	c := github.NewMockClient()
	c.Add(github.RepoData{
		GitHubID: "G1", Owner: "x", Name: "router", StarCount: 50,
		Description: "Fast HTTP router.",
	})
	id, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "router"})
	db.UpsertDailyStar(d, id, "2026-04-17", 5)

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")

	mt := &translate.MockTranslator{
		Responses: map[string]string{"Fast HTTP router.": "高速な HTTP ルーター。"},
	}

	rep, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18",
		SeedOwners: []string{"x"}, SeedNames: []string{"router"},
		OutDir: dataDir, TopN: 100, SkipDiscover: true,
		UpdatedAt: "X", GeneratedAt: "X",
		Translator:          mt,
		TranslationCacheDir: cacheDir,
		TranslateBatchSize:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Translated.Translated != 1 {
		t.Errorf("Translated.Translated = %d, want 1", rep.Translated.Translated)
	}

	// The repo JSON now carries description_ja.
	raw, err := os.ReadFile(filepath.Join(dataDir, "repos", "x__router.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got export.RepoDetail
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DescriptionJA != "高速な HTTP ルーター。" {
		t.Errorf("DescriptionJA = %q, want translated", got.DescriptionJA)
	}
}

// Translator failures must not abort the pipeline. Export still emits
// every repo; description_ja is left empty so the frontend renders the
// English original.
func TestRunAll_TranslatorFailureDoesNotAbort(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	c := github.NewMockClient()
	c.Add(github.RepoData{
		GitHubID: "G1", Owner: "x", Name: "a", StarCount: 50,
		Description: "Fast HTTP router.",
	})
	id, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a"})
	db.UpsertDailyStar(d, id, "2026-04-17", 5)

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")

	mt := &translate.MockTranslator{ForceErr: errAPI}

	rep, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18",
		SeedOwners: []string{"x"}, SeedNames: []string{"a"},
		OutDir: dataDir, TopN: 100, SkipDiscover: true,
		UpdatedAt: "X", GeneratedAt: "X",
		Translator: mt, TranslationCacheDir: cacheDir,
	})
	if err != nil {
		t.Fatalf("translator failure must not propagate: %v", err)
	}
	if rep.Translated.Failed == 0 {
		t.Errorf("expected Failed > 0 in stats, got %+v", rep.Translated)
	}
	if rep.ExportRepos != 1 {
		t.Errorf("export still must run; got ExportRepos=%d", rep.ExportRepos)
	}
}

// Without a Translator but WITH a cache dir, RunAll skips the translate
// step but Export still reads cached entries from prior runs. This
// matches the steady-state CI on a day with no API key configured.
func TestRunAll_SkipsTranslateButReadsExistingCache(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	c := github.NewMockClient()
	c.Add(github.RepoData{
		GitHubID: "G1", Owner: "x", Name: "a", StarCount: 50,
		Description: "Cached one.",
	})
	id, _ := db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a"})
	db.UpsertDailyStar(d, id, "2026-04-17", 5)

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")
	cache := &translate.Cache{Dir: cacheDir}
	if err := cache.Put(translate.CacheEntry{
		Src: "Cached one.", JA: "キャッシュ済み。", Provider: "claude", TranslatedAt: "yesterday",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := RunAll(context.Background(), d, Options{
		Client: c, Today: "2026-04-18",
		SeedOwners: []string{"x"}, SeedNames: []string{"a"},
		OutDir: dataDir, TopN: 100, SkipDiscover: true,
		UpdatedAt: "X", GeneratedAt: "X",
		// Translator deliberately nil.
		TranslationCacheDir: cacheDir,
	}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, "repos", "x__a.json"))
	var got export.RepoDetail
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DescriptionJA != "キャッシュ済み。" {
		t.Errorf("DescriptionJA = %q, want cache hit", got.DescriptionJA)
	}
}

var errAPI = stubError("forced api error")

type stubError string

func (s stubError) Error() string { return string(s) }
