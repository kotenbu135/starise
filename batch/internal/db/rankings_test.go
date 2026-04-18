package db

import (
	"testing"
)

func TestReplaceRankingsForSlot(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	id1, _ := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "x"})
	id2, _ := UpsertRepository(d, Repository{GitHubID: "G2", Owner: "a", Name: "y"})

	rs := []Ranking{
		{RepoID: id1, Period: "1d", RankType: "breakout", ComputedDate: "2026-04-18", StartStars: 5, EndStars: 100, StarDelta: 95, GrowthPct: 1900, Rank: 1},
		{RepoID: id2, Period: "1d", RankType: "breakout", ComputedDate: "2026-04-18", StartStars: 10, EndStars: 50, StarDelta: 40, GrowthPct: 400, Rank: 2},
	}
	if err := ReplaceRankings(d, "1d", "breakout", "2026-04-18", rs); err != nil {
		t.Fatal(err)
	}

	got, _ := ListRankings(d, "1d", "breakout", "2026-04-18")
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Rank != 1 || got[1].Rank != 2 {
		t.Errorf("rank order")
	}

	// Replace clears prior entries for the same slot
	rs2 := []Ranking{
		{RepoID: id1, Period: "1d", RankType: "breakout", ComputedDate: "2026-04-18", StartStars: 5, EndStars: 200, StarDelta: 195, GrowthPct: 3900, Rank: 1},
	}
	if err := ReplaceRankings(d, "1d", "breakout", "2026-04-18", rs2); err != nil {
		t.Fatal(err)
	}
	got2, _ := ListRankings(d, "1d", "breakout", "2026-04-18")
	if len(got2) != 1 {
		t.Errorf("replace did not clear: len=%d", len(got2))
	}
}

func TestReplaceRankingsIsolatesSlots(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	id1, _ := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "x"})
	a := []Ranking{{RepoID: id1, Period: "1d", RankType: "breakout", ComputedDate: "2026-04-18", EndStars: 100, StarDelta: 95, GrowthPct: 1900, Rank: 1}}
	b := []Ranking{{RepoID: id1, Period: "7d", RankType: "trending", ComputedDate: "2026-04-18", StartStars: 100, EndStars: 200, StarDelta: 100, GrowthPct: 100, Rank: 1}}
	ReplaceRankings(d, "1d", "breakout", "2026-04-18", a)
	ReplaceRankings(d, "7d", "trending", "2026-04-18", b)

	// Replacing 1d/breakout must NOT touch 7d/trending
	ReplaceRankings(d, "1d", "breakout", "2026-04-18", nil)
	got, _ := ListRankings(d, "7d", "trending", "2026-04-18")
	if len(got) != 1 {
		t.Errorf("cross-slot leak: 7d/trending len=%d", len(got))
	}
}
