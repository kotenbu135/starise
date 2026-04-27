package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// rankings.json entries must carry description, description_ja, and
// created_at so the frontend can render the home page without scanning
// every file under data/repos/. With ~60k repos that scan dominates
// page-load time.
func TestExportRankings_EnrichesDescriptionAndCreatedAt(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	id, err := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "router",
		Description: "Fast HTTP router.",
		Language:    "Go",
		CreatedAt:   "2024-01-15T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertDailyStar(d, id, "2026-04-17", 5); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertDailyStar(d, id, "2026-04-18", 200); err != nil {
		t.Fatal(err)
	}
	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")
	c := &translate.Cache{Dir: cacheDir}
	if err := c.Put(translate.CacheEntry{
		Src: "Fast HTTP router.", JA: "高速な HTTP ルーター。",
		Provider: "claude", TranslatedAt: "2026-04-26T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := Export(d, Options{
		OutDir: dataDir, UpdatedAt: "X", GeneratedAt: "X",
		ComputedDate: "2026-04-18", TopN: 100,
		TranslationCacheDir: cacheDir,
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rk Rankings
	if err := json.Unmarshal(raw, &rk); err != nil {
		t.Fatal(err)
	}

	// Must land in the trending bucket (start_stars >= 100? actually
	// start was 5 so this is breakout). Either way, check both.
	var found *RankingEntry
	for _, key := range AllRankingKeys() {
		for i := range rk.Rankings[key] {
			if rk.Rankings[key][i].FullName == "x/router" {
				found = &rk.Rankings[key][i]
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		t.Fatal("entry x/router not found in any ranking slot")
	}
	if found.Description != "Fast HTTP router." {
		t.Errorf("Description = %q", found.Description)
	}
	if found.DescriptionJA != "高速な HTTP ルーター。" {
		t.Errorf("DescriptionJA = %q (want translated)", found.DescriptionJA)
	}
	if found.CreatedAt != "2024-01-15T00:00:00Z" {
		t.Errorf("CreatedAt = %q", found.CreatedAt)
	}
}

// description_ja in rankings is empty on cache miss; description still
// populated so frontend can fall back to English.
func TestExportRankings_DescriptionJAEmptyOnCacheMiss(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()

	id, _ := db.UpsertRepository(d, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "y",
		Description: "untranslated", CreatedAt: "2024-01-15T00:00:00Z",
	})
	db.UpsertDailyStar(d, id, "2026-04-17", 5)
	db.UpsertDailyStar(d, id, "2026-04-18", 200)
	_ = ranking.Compute(d, "2026-04-18", 100)

	dataDir := t.TempDir()
	cacheDir := filepath.Join(dataDir, "translations")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Export(d, Options{
		OutDir: dataDir, UpdatedAt: "X", GeneratedAt: "X",
		ComputedDate: "2026-04-18", TopN: 100,
		TranslationCacheDir: cacheDir,
	}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, "rankings.json"))
	var rk Rankings
	json.Unmarshal(raw, &rk)

	var found *RankingEntry
	for _, key := range AllRankingKeys() {
		for i := range rk.Rankings[key] {
			if rk.Rankings[key][i].FullName == "x/y" {
				found = &rk.Rankings[key][i]
				break
			}
		}
	}
	if found == nil {
		t.Fatal("entry not found")
	}
	if found.Description != "untranslated" {
		t.Errorf("Description = %q", found.Description)
	}
	if found.DescriptionJA != "" {
		t.Errorf("DescriptionJA should be empty on miss, got %q", found.DescriptionJA)
	}
}
