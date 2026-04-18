package ranking

import (
	"math"
	"strings"
	"testing"
)

func TestValidateAcceptsCleanSlot(t *testing.T) {
	rs := []Scored{
		{RepoID: 1, StarDelta: 100, GrowthPct: 200, Rank: 1},
		{RepoID: 2, StarDelta: 50, GrowthPct: 100, Rank: 2},
	}
	if err := Validate(rs, RankTypeBreakout); err != nil {
		t.Errorf("clean breakout rejected: %v", err)
	}
}

// I5c
func TestValidateRejectsZeroOrNegativeMetric(t *testing.T) {
	cases := []struct {
		name string
		rs   []Scored
		rt   string
	}{
		{"breakout delta=0", []Scored{{RepoID: 1, StarDelta: 0, GrowthPct: 5, Rank: 1}}, RankTypeBreakout},
		{"breakout delta<0", []Scored{{RepoID: 1, StarDelta: -1, GrowthPct: 5, Rank: 1}}, RankTypeBreakout},
		{"trending growth=0", []Scored{{RepoID: 1, StarDelta: 1, GrowthPct: 0, Rank: 1}}, RankTypeTrending},
		{"trending growth<0", []Scored{{RepoID: 1, StarDelta: 1, GrowthPct: -1, Rank: 1}}, RankTypeTrending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.rs, tc.rt); err == nil {
				t.Errorf("expected rejection")
			}
		})
	}
}

// I6
func TestValidateRejectsNaNAndInf(t *testing.T) {
	cases := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, v := range cases {
		rs := []Scored{{RepoID: 1, StarDelta: 5, GrowthPct: v, Rank: 1}}
		if err := Validate(rs, RankTypeBreakout); err == nil {
			t.Errorf("growth_pct=%v accepted", v)
		}
	}
}

// I7: rank must be 1..N, no gaps, no duplicates.
func TestValidateRejectsBadRankSequence(t *testing.T) {
	cases := map[string][]Scored{
		"start at 0":    {{RepoID: 1, StarDelta: 5, GrowthPct: 1, Rank: 0}},
		"gap":           {{RepoID: 1, StarDelta: 5, GrowthPct: 1, Rank: 1}, {RepoID: 2, StarDelta: 4, GrowthPct: 1, Rank: 3}},
		"dup":           {{RepoID: 1, StarDelta: 5, GrowthPct: 1, Rank: 1}, {RepoID: 2, StarDelta: 4, GrowthPct: 1, Rank: 1}},
		"out of order":  {{RepoID: 1, StarDelta: 5, GrowthPct: 1, Rank: 2}, {RepoID: 2, StarDelta: 4, GrowthPct: 1, Rank: 1}},
	}
	for name, rs := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(rs, RankTypeBreakout); err == nil {
				t.Errorf("expected rejection")
			}
		})
	}
}

// I5d: breakout and trending must not share a repo within the same period.
func TestValidateNoCrossAxisOverlap(t *testing.T) {
	bo := []Scored{{RepoID: 1, StarDelta: 50, GrowthPct: 100, Rank: 1}}
	tr := []Scored{{RepoID: 1, StarDelta: 50, GrowthPct: 100, Rank: 1}}
	if err := ValidateNoOverlap(bo, tr); err == nil {
		t.Errorf("overlap accepted")
	}
	tr2 := []Scored{{RepoID: 2, StarDelta: 50, GrowthPct: 100, Rank: 1}}
	if err := ValidateNoOverlap(bo, tr2); err != nil {
		t.Errorf("disjoint rejected: %v", err)
	}
}

// I12: macro check — at least one slot must contain rows.
func TestMacroValidateRejectsAllEmpty(t *testing.T) {
	all := map[string][]Scored{}
	for _, p := range Periods {
		for _, rt := range RankTypes {
			all[p+"_"+rt] = []Scored{}
		}
	}
	err := MacroValidate(all)
	if err == nil {
		t.Errorf("all-empty accepted")
	}
	if err != nil && !strings.Contains(err.Error(), "empty") {
		t.Errorf("err message should mention emptiness: %v", err)
	}

	all["1d_breakout"] = []Scored{{RepoID: 1, StarDelta: 5, GrowthPct: 1, Rank: 1}}
	if err := MacroValidate(all); err != nil {
		t.Errorf("non-empty rejected: %v", err)
	}
}
