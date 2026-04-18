package pipeline

import (
	"math"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// I6: ranking results never contain NaN, +Inf, or -Inf.
// Note: pure-function path (start>=1) cannot produce these; this end-to-end
// test guards regressions in case future changes alter the formula.
func TestInvariantI6_NoNaNOrInf_Real(t *testing.T) {
	d := openMem(t)
	mustUpsert(t, d, "A", "x", "a", false, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	mustUpsert(t, d, "B", "x", "b", false, false, map[string]int{"2026-04-17": 100, "2026-04-18": 200})

	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}
	for _, period := range ranking.Periods {
		for _, rt := range ranking.RankTypes {
			rs, _ := db.ListRankings(d, period, rt, "2026-04-18")
			for _, r := range rs {
				if math.IsNaN(r.GrowthPct) || math.IsInf(r.GrowthPct, 0) {
					t.Errorf("%s/%s repo %d: growth_pct=%v", period, rt, r.RepoID, r.GrowthPct)
				}
			}
		}
	}
}
