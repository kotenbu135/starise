package ranking

import (
	"database/sql"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seed: repo + history at various dates
func seedRepo(t *testing.T, d *sql.DB, ghID, owner, name string, archived, fork bool, dailyStars map[string]int) int64 {
	t.Helper()
	id, err := db.UpsertRepository(d, db.Repository{
		GitHubID: ghID, Owner: owner, Name: name,
		IsArchived: archived, IsFork: fork,
	})
	if err != nil {
		t.Fatal(err)
	}
	for date, c := range dailyStars {
		if err := db.UpsertDailyStar(d, id, date, c); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestComputeWritesAllSixSlots(t *testing.T) {
	d := openTestDB(t)
	// Two repos: one breakout candidate, one trending candidate.
	seedRepo(t, d, "A", "x", "small", false, false, map[string]int{
		"2026-04-11": 5, "2026-04-17": 5, "2026-04-18": 50, // delta 45 over 1d, 45 over 7d
	})
	seedRepo(t, d, "B", "x", "big", false, false, map[string]int{
		"2026-04-11": 500, "2026-04-17": 500, "2026-04-18": 1000,
	})

	if err := Compute(d, "2026-04-18", 100); err != nil {
		t.Fatalf("compute: %v", err)
	}

	for _, period := range Periods {
		for _, rt := range RankTypes {
			rs, err := db.ListRankings(d, period, rt, "2026-04-18")
			if err != nil {
				t.Fatalf("list %s/%s: %v", period, rt, err)
			}
			if len(rs) == 0 {
				t.Errorf("slot %s/%s empty", period, rt)
			}
		}
	}
}

func TestComputeExcludesArchivedAndFork(t *testing.T) {
	d := openTestDB(t)
	seedRepo(t, d, "ARC", "x", "arc", true, false, map[string]int{
		"2026-04-17": 5, "2026-04-18": 100,
	})
	seedRepo(t, d, "FRK", "x", "frk", false, true, map[string]int{
		"2026-04-17": 5, "2026-04-18": 100,
	})
	seedRepo(t, d, "OK", "x", "ok", false, false, map[string]int{
		"2026-04-17": 5, "2026-04-18": 100,
	})

	if err := Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}
	rs, _ := db.ListRankings(d, "1d", "breakout", "2026-04-18")
	if len(rs) != 1 {
		t.Fatalf("len=%d, want 1 (only OK)", len(rs))
	}
}

func TestComputeExcludesMissingSnapshots(t *testing.T) {
	d := openTestDB(t)
	// has end but no start — must be excluded
	seedRepo(t, d, "A", "x", "a", false, false, map[string]int{"2026-04-18": 500})
	// has start but no end — must be excluded
	seedRepo(t, d, "B", "x", "b", false, false, map[string]int{"2026-04-17": 500})

	Compute(d, "2026-04-18", 100)
	for _, p := range Periods {
		for _, rt := range RankTypes {
			rs, _ := db.ListRankings(d, p, rt, "2026-04-18")
			if len(rs) != 0 {
				t.Errorf("%s/%s has %d rows, want 0", p, rt, len(rs))
			}
		}
	}
}

func TestComputeRespectsTopN(t *testing.T) {
	d := openTestDB(t)
	for i := 0; i < 5; i++ {
		ghID := string(rune('A' + i))
		seedRepo(t, d, ghID, "x", ghID, false, false, map[string]int{
			"2026-04-17": 5, "2026-04-18": 5 + i + 1, // varying delta
		})
	}
	if err := Compute(d, "2026-04-18", 3); err != nil {
		t.Fatal(err)
	}
	rs, _ := db.ListRankings(d, "1d", "breakout", "2026-04-18")
	if len(rs) != 3 {
		t.Errorf("topN=3, got %d", len(rs))
	}
}

func TestComputeReplacesPriorRun(t *testing.T) {
	d := openTestDB(t)
	id := seedRepo(t, d, "A", "x", "a", false, false, map[string]int{
		"2026-04-17": 5, "2026-04-18": 50,
	})
	Compute(d, "2026-04-18", 100)

	// Update star count and re-run
	db.UpsertDailyStar(d, id, "2026-04-18", 200)
	Compute(d, "2026-04-18", 100)

	rs, _ := db.ListRankings(d, "1d", "breakout", "2026-04-18")
	if len(rs) != 1 || rs[0].EndStars != 200 {
		t.Errorf("re-run did not refresh: %+v", rs)
	}
}
