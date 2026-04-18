package ranking

// Candidate is the input row for both axes.
// RepoID is used as the deterministic tie-break key.
type Candidate struct {
	RepoID     int64
	StartStars int
	EndStars   int
}

// Scored is a Candidate plus the ranked entry written to the rankings table.
// StarDelta = EndStars - StartStars.
// GrowthPct = (EndStars - StartStars) / StartStars * 100; 0 for breakout rows
// (since breakout uses StarDelta, GrowthPct is informational only).
// Rank starts at 1.
type Scored struct {
	RepoID     int64
	StartStars int
	EndStars   int
	StarDelta  int
	GrowthPct  float64
	Rank       int
}

const (
	BreakoutMaxStartExclusive = 100 // breakout requires StartStars in [1, 100)
	TrendingMinStartInclusive = 100 // trending requires StartStars >= 100

	RankTypeBreakout = "breakout"
	RankTypeTrending = "trending"
)

var Periods = []string{"1d", "7d", "30d"}
var RankTypes = []string{RankTypeBreakout, RankTypeTrending}
