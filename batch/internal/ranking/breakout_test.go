package ranking

import "testing"

func TestComputeBreakoutFiltersAndOrders(t *testing.T) {
	in := []Candidate{
		{RepoID: 1, StartStars: 5, EndStars: 100},   // delta 95 — breakout
		{RepoID: 2, StartStars: 100, EndStars: 200}, // start>=100 → excluded
		{RepoID: 3, StartStars: 10, EndStars: 50},   // delta 40
		{RepoID: 4, StartStars: 0, EndStars: 50},    // start=0 → excluded
		{RepoID: 5, StartStars: 1, EndStars: 1},     // delta 0 → excluded
		{RepoID: 6, StartStars: 5, EndStars: 1},     // delta -4 → excluded
		{RepoID: 7, StartStars: 99, EndStars: 200},  // delta 101 — included (start<100)
	}
	got := ComputeBreakout(in)
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}
	wantOrder := []int64{7, 1, 3} // delta 101, 95, 40
	for i, w := range wantOrder {
		if got[i].RepoID != w {
			t.Errorf("rank %d: got repo=%d, want %d (delta=%d)", i+1, got[i].RepoID, w, got[i].StarDelta)
		}
		if got[i].Rank != i+1 {
			t.Errorf("rank field idx %d: got %d", i, got[i].Rank)
		}
	}
}

func TestComputeBreakoutTieBreakRepoIDAsc(t *testing.T) {
	in := []Candidate{
		{RepoID: 30, StartStars: 5, EndStars: 50},
		{RepoID: 10, StartStars: 5, EndStars: 50},
		{RepoID: 20, StartStars: 5, EndStars: 50},
	}
	got := ComputeBreakout(in)
	want := []int64{10, 20, 30}
	for i, w := range want {
		if got[i].RepoID != w {
			t.Errorf("idx %d: got %d, want %d", i, got[i].RepoID, w)
		}
	}
}

func TestComputeBreakoutBoundary(t *testing.T) {
	// start==100 → excluded (trending side)
	got := ComputeBreakout([]Candidate{{RepoID: 1, StartStars: 100, EndStars: 200}})
	if len(got) != 0 {
		t.Errorf("start==100 should be excluded from breakout, got %d", len(got))
	}
	// start==99 → included
	got = ComputeBreakout([]Candidate{{RepoID: 1, StartStars: 99, EndStars: 100}})
	if len(got) != 1 {
		t.Errorf("start==99 should be included in breakout, got %d", len(got))
	}
}
