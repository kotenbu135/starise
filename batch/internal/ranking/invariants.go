package ranking

import (
	"fmt"
	"math"
)

// Validate checks an in-memory slot for invariants:
//   - I5c: metric > 0 (delta for breakout, growth_pct for trending)
//   - I6:  no NaN / Inf in growth_pct
//   - I7:  rank values are 1..N contiguous, no duplicates, in order
func Validate(rs []Scored, rankType string) error {
	for i, r := range rs {
		if math.IsNaN(r.GrowthPct) || math.IsInf(r.GrowthPct, 0) {
			return fmt.Errorf("ranking idx %d: growth_pct=%v (NaN/Inf)", i, r.GrowthPct)
		}
		switch rankType {
		case RankTypeBreakout:
			if r.StarDelta <= 0 {
				return fmt.Errorf("ranking idx %d: breakout star_delta=%d (must be > 0)", i, r.StarDelta)
			}
		case RankTypeTrending:
			if r.GrowthPct <= 0 {
				return fmt.Errorf("ranking idx %d: trending growth_pct=%v (must be > 0)", i, r.GrowthPct)
			}
		default:
			return fmt.Errorf("unknown rank_type %q", rankType)
		}
		if r.Rank != i+1 {
			return fmt.Errorf("ranking idx %d: rank=%d (expected %d, sequence must be 1..N)", i, r.Rank, i+1)
		}
	}
	return nil
}

// ValidateNoOverlap enforces I5d: a repo must not appear in both axes for
// the same period.
func ValidateNoOverlap(breakout, trending []Scored) error {
	in := make(map[int64]bool, len(breakout))
	for _, r := range breakout {
		in[r.RepoID] = true
	}
	for _, r := range trending {
		if in[r.RepoID] {
			return fmt.Errorf("repo %d appears in both breakout and trending", r.RepoID)
		}
	}
	return nil
}

// MacroValidate enforces I12: at least one slot must have rows.
// Keys are "<period>_<rank_type>" (e.g. "1d_breakout").
func MacroValidate(slots map[string][]Scored) error {
	total := 0
	for _, rs := range slots {
		total += len(rs)
	}
	if total == 0 {
		return fmt.Errorf("all ranking slots empty — pipeline produced no output")
	}
	return nil
}
