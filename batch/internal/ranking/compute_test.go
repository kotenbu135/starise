package ranking

import (
	"math"
	"testing"
)

// TestGrowthPctFormula locks down the core math:
//   growth = (end - start) / start * 100   (in %)
// Below MinStartStars → result is excluded. start == 0 → excluded.
func TestGrowthPctFormula(t *testing.T) {
	cases := []struct {
		name     string
		start    int
		end      int
		wantPct  float64
		excluded bool
	}{
		{"normal growth", 100, 150, 50.0, false},
		{"no change", 100, 100, 0.0, false},
		{"decline", 100, 80, -20.0, false},
		{"threshold exact", 10, 20, 100.0, false},
		{"above threshold", 50, 75, 50.0, false},
		{"zero start excluded", 0, 100, 0, true},
		{"below threshold excluded", 5, 100, 0, true},
		{"one star excluded", 1, 100, 0, true},
		{"nine stars excluded", 9, 9999, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := GrowthPct(tc.start, tc.end)
			if ok == tc.excluded {
				t.Fatalf("inclusion mismatch: ok=%v excluded=%v (start=%d end=%d)",
					ok, tc.excluded, tc.start, tc.end)
			}
			if !tc.excluded && !floatEqual(got, tc.wantPct, 1e-9) {
				t.Errorf("pct mismatch: got %v want %v", got, tc.wantPct)
			}
		})
	}
}

func TestGrowthPctNeverReturnsNaNOrInf(t *testing.T) {
	// Fuzz-ish: every input that is included must yield a finite float.
	for start := 0; start <= 30; start++ {
		for end := 0; end <= 1000; end += 37 {
			got, ok := GrowthPct(start, end)
			if !ok {
				continue
			}
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("start=%d end=%d produced non-finite %v", start, end, got)
			}
		}
	}
}

// TestComputeRepoGrowth verifies the window-based growth computation using
// daily snapshots. start snapshot is the latest at-or-before (endDate - period).
func TestComputeRepoGrowth(t *testing.T) {
	snapshots := []Snapshot{
		{Date: "2026-03-18", Stars: 100},
		{Date: "2026-04-10", Stars: 150},
		{Date: "2026-04-11", Stars: 160},
		{Date: "2026-04-17", Stars: 200},
		{Date: "2026-04-18", Stars: 210},
	}

	cases := []struct {
		name     string
		period   Period
		wantPct  float64
		wantDelt int
		wantStart int
		wantEnd   int
		excluded bool
	}{
		// 1d: base = 2026-04-17 (200). end = 2026-04-18 (210). (210-200)/200*100 = 5
		{"1d", Period1d, 5.0, 10, 200, 210, false},
		// 7d: base = 2026-04-11 (160). (210-160)/160*100 = 31.25
		{"7d", Period7d, 31.25, 50, 160, 210, false},
		// 30d: base = 2026-03-18 snapshot (at-or-before 2026-03-19) -> 100. (210-100)/100 = 110
		{"30d", Period30d, 110.0, 110, 100, 210, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := ComputeRepoGrowth(snapshots, "2026-04-18", tc.period)
			if !ok {
				t.Fatalf("unexpected exclusion")
			}
			if r.StartStars != tc.wantStart || r.EndStars != tc.wantEnd {
				t.Errorf("range wrong: start=%d end=%d want start=%d end=%d",
					r.StartStars, r.EndStars, tc.wantStart, tc.wantEnd)
			}
			if r.StarDelta != tc.wantDelt {
				t.Errorf("delta: %d want %d", r.StarDelta, tc.wantDelt)
			}
			if !floatEqual(r.GrowthPct, tc.wantPct, 1e-9) {
				t.Errorf("pct: %v want %v", r.GrowthPct, tc.wantPct)
			}
		})
	}
}

func TestComputeRepoGrowthMissingEndSnapshot(t *testing.T) {
	// end date has no snapshot at or before → excluded.
	snapshots := []Snapshot{{Date: "2026-05-01", Stars: 100}}
	_, ok := ComputeRepoGrowth(snapshots, "2026-04-18", Period7d)
	if ok {
		t.Fatal("expected exclusion: no end snapshot")
	}
}

func TestComputeRepoGrowthMissingStartSnapshot(t *testing.T) {
	// no snapshot at-or-before the period start → excluded.
	// End date 2026-04-18. 30d back = 2026-03-19. Only snapshot is 2026-04-18.
	snapshots := []Snapshot{{Date: "2026-04-18", Stars: 500}}
	_, ok := ComputeRepoGrowth(snapshots, "2026-04-18", Period30d)
	if ok {
		t.Fatal("expected exclusion: no start snapshot")
	}
}

func TestComputeRepoGrowthBelowMinStart(t *testing.T) {
	snapshots := []Snapshot{
		{Date: "2026-04-11", Stars: 5},
		{Date: "2026-04-18", Stars: 500},
	}
	_, ok := ComputeRepoGrowth(snapshots, "2026-04-18", Period7d)
	if ok {
		t.Fatal("expected exclusion: start 5 below MinStartStars 10")
	}
}

func TestRankAssignsDenseByPctDesc(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, GrowthPct: 10},
		{RepoID: 2, GrowthPct: 50},
		{RepoID: 3, GrowthPct: 30},
	}
	ranked := AssignRanks(rows)
	// Expect rank 1 = repo 2 (50%), rank 2 = repo 3 (30%), rank 3 = repo 1 (10%)
	if ranked[0].RepoID != 2 || ranked[0].Rank != 1 {
		t.Errorf("rank1: %+v", ranked[0])
	}
	if ranked[1].RepoID != 3 || ranked[1].Rank != 2 {
		t.Errorf("rank2: %+v", ranked[1])
	}
	if ranked[2].RepoID != 1 || ranked[2].Rank != 3 {
		t.Errorf("rank3: %+v", ranked[2])
	}
}

func TestRankStableTieBreakByRepoID(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 5, GrowthPct: 50.0},
		{RepoID: 2, GrowthPct: 50.0},
		{RepoID: 9, GrowthPct: 50.0},
	}
	ranked := AssignRanks(rows)
	if ranked[0].RepoID != 2 || ranked[1].RepoID != 5 || ranked[2].RepoID != 9 {
		t.Errorf("tie break expected ascending repo id, got %+v", ranked)
	}
}

func floatEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}
