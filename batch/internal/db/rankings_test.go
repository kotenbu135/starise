package db

import (
	"testing"
)

func TestReplaceRankingsForDateRemovesOldAndInserts(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoA, _ := UpsertRepository(d, &Repository{GitHubID: "ga", Owner: "o", Name: "a"})
	repoB, _ := UpsertRepository(d, &Repository{GitHubID: "gb", Owner: "o", Name: "b"})

	// Seed stale data for the target (period, date) pair.
	stale := []Ranking{
		{RepoID: repoA, Period: "7d", ComputedDate: "2026-04-18", StartStars: 0, EndStars: 0, StarDelta: 0, GrowthPct: 999, Rank: 1},
	}
	if err := ReplaceRankingsForDate(d, "7d", "2026-04-18", stale); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fresh := []Ranking{
		{RepoID: repoA, Period: "7d", ComputedDate: "2026-04-18", StartStars: 100, EndStars: 150, StarDelta: 50, GrowthPct: 50.0, Rank: 2},
		{RepoID: repoB, Period: "7d", ComputedDate: "2026-04-18", StartStars: 100, EndStars: 250, StarDelta: 150, GrowthPct: 150.0, Rank: 1},
	}
	if err := ReplaceRankingsForDate(d, "7d", "2026-04-18", fresh); err != nil {
		t.Fatalf("replace: %v", err)
	}

	list, err := ListRankings(d, "7d", "2026-04-18", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d want 2", len(list))
	}
	if list[0].Rank != 1 || list[0].RepoID != repoB {
		t.Errorf("expected repoB at rank 1, got repo=%d rank=%d", list[0].RepoID, list[0].Rank)
	}
	if list[1].GrowthPct != 50.0 {
		t.Errorf("repoA growth: %v", list[1].GrowthPct)
	}
}

func TestReplaceRankingsScopedByPeriod(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repo, _ := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})

	_ = ReplaceRankingsForDate(d, "1d", "2026-04-18", []Ranking{
		{RepoID: repo, Period: "1d", ComputedDate: "2026-04-18", Rank: 1, GrowthPct: 10},
	})
	_ = ReplaceRankingsForDate(d, "7d", "2026-04-18", []Ranking{
		{RepoID: repo, Period: "7d", ComputedDate: "2026-04-18", Rank: 1, GrowthPct: 70},
	})

	// Replacing 7d should not affect 1d rows.
	_ = ReplaceRankingsForDate(d, "7d", "2026-04-18", []Ranking{
		{RepoID: repo, Period: "7d", ComputedDate: "2026-04-18", Rank: 1, GrowthPct: 71},
	})

	d1, _ := ListRankings(d, "1d", "2026-04-18", 10)
	d7, _ := ListRankings(d, "7d", "2026-04-18", 10)
	if len(d1) != 1 || d1[0].GrowthPct != 10 {
		t.Errorf("1d disturbed: %+v", d1)
	}
	if len(d7) != 1 || d7[0].GrowthPct != 71 {
		t.Errorf("7d not updated: %+v", d7)
	}
}

func TestListRankingsRespectsLimit(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	// Need distinct repos because (repo_id, period, computed_date) is UNIQUE.
	var uniq []Ranking
	for i := 1; i <= 5; i++ {
		r, err := UpsertRepository(d, &Repository{
			GitHubID: "gid" + string(rune('0'+i)),
			Owner:    "o", Name: "r" + string(rune('0'+i)),
		})
		if err != nil {
			t.Fatalf("seed repo: %v", err)
		}
		uniq = append(uniq, Ranking{
			RepoID: r, Period: "7d", ComputedDate: "2026-04-18",
			Rank: i, GrowthPct: float64(100 - i),
		})
	}
	if err := ReplaceRankingsForDate(d, "7d", "2026-04-18", uniq); err != nil {
		t.Fatalf("seed: %v", err)
	}

	list, err := ListRankings(d, "7d", "2026-04-18", 3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("limit not honored: %d", len(list))
	}
	if list[0].Rank != 1 || list[2].Rank != 3 {
		t.Errorf("rank order wrong: %+v", list)
	}
}
