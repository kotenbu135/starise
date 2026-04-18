package ranking

import "sort"

// ComputeTrending applies the trending axis: start >= 100, growth_pct > 0.
// Sorted by GrowthPct DESC, tie-break RepoID ASC. Ranks assigned 1..N.
func ComputeTrending(in []Candidate) []Scored {
	out := make([]Scored, 0, len(in))
	for _, c := range in {
		if c.StartStars < TrendingMinStartInclusive {
			continue
		}
		if c.EndStars <= c.StartStars {
			continue
		}
		delta := c.EndStars - c.StartStars
		// StartStars >= 100 here, so division is safe and finite.
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
		if out[i].GrowthPct != out[j].GrowthPct {
			return out[i].GrowthPct > out[j].GrowthPct
		}
		return out[i].RepoID < out[j].RepoID
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}
