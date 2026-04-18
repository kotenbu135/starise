package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/export"
)

// I8: rankings.json always carries the 6 keys, even when slots are empty.
func TestInvariantI8_RankingsJSONHasSixKeys_Real(t *testing.T) {
	d := openMem(t)
	dir := t.TempDir()
	if _, err := export.Export(d, export.Options{
		OutDir: dir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "rankings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rk export.Rankings
	if err := json.Unmarshal(b, &rk); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, 6)
	for k := range rk.Rankings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"1d_breakout", "1d_trending", "30d_breakout", "30d_trending", "7d_breakout", "7d_trending"}
	if len(keys) != 6 {
		t.Errorf("got %d keys, want 6: %v", len(keys), keys)
	}
	for _, w := range want {
		found := false
		for _, k := range keys {
			if k == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing key %s", w)
		}
	}
}
