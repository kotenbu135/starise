package ranking

import (
	"math"
	"testing"
)

func TestComputeTrendingFiltersAndOrders(t *testing.T) {
	in := []Candidate{
		{RepoID: 1, StartStars: 100, EndStars: 200},  // 100% — trending
		{RepoID: 2, StartStars: 99, EndStars: 198},   // start<100 → excluded
		{RepoID: 3, StartStars: 1000, EndStars: 1500}, // 50%
		{RepoID: 4, StartStars: 100, EndStars: 100},  // growth=0 → excluded
		{RepoID: 5, StartStars: 100, EndStars: 99},   // negative → excluded
		{RepoID: 6, StartStars: 200, EndStars: 600},  // 200%
	}
	got := ComputeTrending(in)
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}

	wantOrder := []int64{6, 1, 3} // 200%, 100%, 50%
	for i, w := range wantOrder {
		if got[i].RepoID != w {
			t.Errorf("rank %d: got repo=%d, want %d", i+1, got[i].RepoID, w)
		}
		if got[i].Rank != i+1 {
			t.Errorf("rank field: idx %d got %d", i, got[i].Rank)
		}
	}

	// growth_pct correctness
	if math.Abs(got[0].GrowthPct-200.0) > 1e-9 {
		t.Errorf("rank1 growth=%v", got[0].GrowthPct)
	}
}

func TestComputeTrendingTieBreakRepoIDAsc(t *testing.T) {
	in := []Candidate{
		{RepoID: 30, StartStars: 100, EndStars: 200},
		{RepoID: 10, StartStars: 100, EndStars: 200},
		{RepoID: 20, StartStars: 100, EndStars: 200},
	}
	got := ComputeTrending(in)
	want := []int64{10, 20, 30}
	for i, w := range want {
		if got[i].RepoID != w {
			t.Errorf("idx %d: got %d, want %d", i, got[i].RepoID, w)
		}
	}
}

func TestComputeTrendingExcludesAtBoundary(t *testing.T) {
	// start == 100 is INCLUDED in trending (per issue spec)
	in := []Candidate{{RepoID: 1, StartStars: 100, EndStars: 101}}
	got := ComputeTrending(in)
	if len(got) != 1 {
		t.Errorf("start==100 should be included, got %d rows", len(got))
	}
}

func TestComputeTrendingNoNaNOrInf(t *testing.T) {
	in := []Candidate{
		{RepoID: 1, StartStars: 0, EndStars: 100}, // would be Inf — must be excluded
	}
	got := ComputeTrending(in)
	if len(got) != 0 {
		t.Errorf("start=0 must be excluded, got %d", len(got))
	}
	for _, r := range got {
		if math.IsInf(r.GrowthPct, 0) || math.IsNaN(r.GrowthPct) {
			t.Errorf("inf/nan leaked: %v", r.GrowthPct)
		}
	}
}
