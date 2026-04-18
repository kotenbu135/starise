package ranking

import "sort"

// ComputeBreakout applies the breakout axis: 1 <= start < 100 AND delta > 0.
// Sorted by StarDelta DESC, tie-break RepoID ASC. Ranks assigned 1..N.
// GrowthPct is computed for informational purposes (always finite given start >= 1).
func ComputeBreakout(in []Candidate) []Scored {
	out := make([]Scored, 0, len(in))
	for _, c := range in {
		if c.StartStars < 1 || c.StartStars >= BreakoutMaxStartExclusive {
			continue
		}
		delta := c.EndStars - c.StartStars
		if delta <= 0 {
			continue
		}
		growth := float64(delta) / float64(c.StartStars) * 100.0
		out = append(out, Scored{
			RepoID:     c.RepoID,
			StartStars: c.StartStars,
			EndStars:   c.EndStars,
			StarDelta:  delta,
			GrowthPct:  growth,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StarDelta != out[j].StarDelta {
			return out[i].StarDelta > out[j].StarDelta
		}
		return out[i].RepoID < out[j].RepoID
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}
