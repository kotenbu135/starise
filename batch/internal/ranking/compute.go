// Package ranking computes %-growth rankings from daily star snapshots.
//
// Core formula (documented and unit-tested):
//
//	growth_pct = (end_stars - start_stars) / start_stars * 100
//
// Exclusion rules (intentional — see CLAUDE.md):
//
//  1. start_stars < MinStartStars  → excluded (statistical noise: 1→2 = +100%)
//  2. no snapshot at-or-before period start → excluded (insufficient data)
//  3. no snapshot at-or-before period end   → excluded (insufficient data)
//
// These are the only paths to exclusion. The function is a pure function of
// its inputs and must never panic / return NaN / return Inf.
package ranking

import (
	"sort"
	"time"
)

// MinStartStars is the minimum start_stars required for a repo to appear in
// the %-growth ranking. Below this, tiny denominators produce misleading %.
const MinStartStars = 10

// Period identifies a ranking window.
type Period string

const (
	Period1d  Period = "1d"
	Period7d  Period = "7d"
	Period30d Period = "30d"
)

// Days returns the number of days in the window.
func (p Period) Days() int {
	switch p {
	case Period1d:
		return 1
	case Period7d:
		return 7
	case Period30d:
		return 30
	}
	return 0
}

// Snapshot is a (date, stars) pair. Date is YYYY-MM-DD.
type Snapshot struct {
	Date  string
	Stars int
}

// RepoGrowth is the result of computing growth for one repo+period.
type RepoGrowth struct {
	RepoID     int64
	Period     Period
	StartStars int
	EndStars   int
	StarDelta  int
	GrowthPct  float64
	Rank       int // populated by AssignRanks
}

// GrowthPct applies the core formula with exclusion rules.
// Returns (pct, true) when included, or (0, false) when excluded.
func GrowthPct(start, end int) (float64, bool) {
	if start < MinStartStars {
		return 0, false
	}
	return float64(end-start) / float64(start) * 100, true
}

// ComputeRepoGrowth derives a RepoGrowth from snapshots (ascending by date).
// endDate is the "today" anchor (YYYY-MM-DD). period defines the window.
func ComputeRepoGrowth(snapshots []Snapshot, endDate string, period Period) (RepoGrowth, bool) {
	end, ok := latestAtOrBefore(snapshots, endDate)
	if !ok {
		return RepoGrowth{}, false
	}

	startDate, err := subtractDays(endDate, period.Days())
	if err != nil {
		return RepoGrowth{}, false
	}
	start, ok := latestAtOrBefore(snapshots, startDate)
	if !ok {
		return RepoGrowth{}, false
	}

	pct, included := GrowthPct(start.Stars, end.Stars)
	if !included {
		return RepoGrowth{}, false
	}
	return RepoGrowth{
		Period:     period,
		StartStars: start.Stars,
		EndStars:   end.Stars,
		StarDelta:  end.Stars - start.Stars,
		GrowthPct:  pct,
	}, true
}

// AssignRanks sorts rows by GrowthPct desc, RepoID asc (stable tie-break),
// and writes 1-indexed Rank into each row.
func AssignRanks(rows []RepoGrowth) []RepoGrowth {
	out := make([]RepoGrowth, len(rows))
	copy(out, rows)

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

// latestAtOrBefore returns the snapshot with the greatest Date <= target.
// Assumes snapshots may be in any order; performs a linear scan.
func latestAtOrBefore(snapshots []Snapshot, target string) (Snapshot, bool) {
	var best Snapshot
	found := false
	for _, s := range snapshots {
		if s.Date > target {
			continue
		}
		if !found || s.Date > best.Date {
			best = s
			found = true
		}
	}
	return best, found
}

func subtractDays(date string, days int) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, -days).Format("2006-01-02"), nil
}
