package ranking

import (
	"fmt"
	"math"
)

// Validate runs a set of post-compute sanity checks on a per-period ranking.
// Any violation returns an error; the calling CLI should treat this as fatal
// (exit 1). Checks are intentionally conservative: if any row looks wrong,
// we do not ship the data.
//
// Checks:
//
//  1. rows is non-empty
//  2. no NaN / +/-Inf in GrowthPct
//  3. StartStars >= MinStartStars (exclusion rules were applied upstream)
//  4. StarDelta == EndStars - StartStars (arithmetic consistency)
//  5. rank sequence is 1..N with no duplicates
//  6. rank ordering matches GrowthPct desc (higher rank → higher pct allowed,
//     but a row with higher pct must never have a higher rank number than
//     one with lower pct, accounting for ties)
// MacroValidate asserts that at least one period has non-empty rankings.
// Use this after aggregating every period's result — if the entire run
// produced no rankings, something upstream is broken.
func MacroValidate(byPeriod map[Period][]RepoGrowth) error {
	for _, rows := range byPeriod {
		if len(rows) > 0 {
			return nil
		}
	}
	return fmt.Errorf("all periods empty: no rankings produced")
}

func Validate(rows []RepoGrowth) error {
	if len(rows) == 0 {
		// An empty period is legitimate (e.g. 30d window with <30 days of
		// history). Macro-level emptiness is enforced by the caller.
		return nil
	}

	seenRank := make(map[int]int64, len(rows))
	for i, r := range rows {
		if math.IsNaN(r.GrowthPct) {
			return fmt.Errorf("row %d repo=%d growth_pct is NaN", i, r.RepoID)
		}
		if math.IsInf(r.GrowthPct, 0) {
			return fmt.Errorf("row %d repo=%d growth_pct is Inf", i, r.RepoID)
		}
		if r.StartStars < MinStartStars {
			return fmt.Errorf("row %d repo=%d start_stars=%d below MinStartStars=%d",
				i, r.RepoID, r.StartStars, MinStartStars)
		}
		if got := r.EndStars - r.StartStars; r.StarDelta != got {
			return fmt.Errorf("row %d repo=%d delta inconsistency: star_delta=%d but end-start=%d",
				i, r.RepoID, r.StarDelta, got)
		}
		if prior, ok := seenRank[r.Rank]; ok {
			return fmt.Errorf("duplicate rank %d for repos %d and %d", r.Rank, prior, r.RepoID)
		}
		seenRank[r.Rank] = r.RepoID
	}

	// Rank monotonicity: after sorting by rank ascending, growth_pct must be
	// monotonically non-increasing. Scan the caller's order with rank map.
	byRank := make(map[int]RepoGrowth, len(rows))
	for _, r := range rows {
		byRank[r.Rank] = r
	}
	for n := 1; n < len(rows); n++ {
		cur, curOK := byRank[n]
		nxt, nxtOK := byRank[n+1]
		if !curOK || !nxtOK {
			return fmt.Errorf("rank sequence has gap: expected 1..%d contiguous", len(rows))
		}
		if nxt.GrowthPct > cur.GrowthPct {
			return fmt.Errorf("rank ordering violated: rank %d pct=%v > rank %d pct=%v",
				n+1, nxt.GrowthPct, n, cur.GrowthPct)
		}
	}
	return nil
}
