package pipeline

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// I7: per (period, rank_type), ranks form a contiguous 1..N with no gaps or duplicates.
func TestInvariantI7_RankSequenceContiguous_Real(t *testing.T) {
	d := openMem(t)
	for i := 0; i < 5; i++ {
		ghID := string(rune('A' + i))
		mustUpsert(t, d, ghID, "x", ghID, false, false, map[string]int{
			"2026-04-17": 5, "2026-04-18": 5 + i + 1,
		})
	}
	for i := 0; i < 4; i++ {
		ghID := "T" + string(rune('A'+i))
		mustUpsert(t, d, ghID, "x", ghID, false, false, map[string]int{
			"2026-04-17": 100, "2026-04-18": 100 * (i + 2),
		})
	}

	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}

	for _, period := range ranking.Periods {
		for _, rt := range ranking.RankTypes {
			rs, _ := db.ListRankings(d, period, rt, "2026-04-18")
			seen := map[int]bool{}
			for i, r := range rs {
				if r.Rank != i+1 {
					t.Errorf("%s/%s idx %d: rank=%d, want %d", period, rt, i, r.Rank, i+1)
				}
				if seen[r.Rank] {
					t.Errorf("%s/%s duplicate rank %d", period, rt, r.Rank)
				}
				seen[r.Rank] = true
			}
		}
	}
}
