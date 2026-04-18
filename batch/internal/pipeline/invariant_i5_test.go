package pipeline

import (
	"database/sql"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	_ "modernc.org/sqlite"
)

func openMem(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func mustUpsert(t *testing.T, d *sql.DB, ghID, owner, name string, archived, fork bool, hist map[string]int) int64 {
	t.Helper()
	id, err := db.UpsertRepository(d, db.Repository{
		GitHubID: ghID, Owner: owner, Name: name,
		IsArchived: archived, IsFork: fork,
	})
	if err != nil {
		t.Fatal(err)
	}
	for date, n := range hist {
		if err := db.UpsertDailyStar(d, id, date, n); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// I5a: breakout follows 1 <= start < 100, delta > 0, sorted by delta desc.
// I5b: trending follows start >= 100, growth_pct > 0, sorted by growth_pct desc.
// I5c: no zero or negative metrics survive into rankings.
// I5d: no repo appears in both axes within the same period.
func TestInvariantI5_TwoAxisCorrectness_Real(t *testing.T) {
	d := openMem(t)

	// breakout candidates
	mustUpsert(t, d, "B1", "x", "b1", false, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	mustUpsert(t, d, "B2", "x", "b2", false, false, map[string]int{"2026-04-17": 10, "2026-04-18": 50})
	// trending candidates
	mustUpsert(t, d, "T1", "x", "t1", false, false, map[string]int{"2026-04-17": 100, "2026-04-18": 300})
	mustUpsert(t, d, "T2", "x", "t2", false, false, map[string]int{"2026-04-17": 1000, "2026-04-18": 1500})
	// excluded: archived
	mustUpsert(t, d, "A", "x", "a", true, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	// excluded: fork
	mustUpsert(t, d, "F", "x", "f", false, true, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	// excluded: no growth
	mustUpsert(t, d, "Z", "x", "z", false, false, map[string]int{"2026-04-17": 50, "2026-04-18": 50})

	if err := ranking.Compute(d, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}

	bo, _ := db.ListRankings(d, "1d", "breakout", "2026-04-18")
	tr, _ := db.ListRankings(d, "1d", "trending", "2026-04-18")

	// I5a: breakout filter
	for _, r := range bo {
		if r.StartStars < 1 || r.StartStars >= 100 {
			t.Errorf("breakout repo %d violated start range: %d", r.RepoID, r.StartStars)
		}
		if r.StarDelta <= 0 {
			t.Errorf("breakout repo %d delta <= 0: %d", r.RepoID, r.StarDelta)
		}
	}
	// breakout sorted by delta desc
	for i := 1; i < len(bo); i++ {
		if bo[i-1].StarDelta < bo[i].StarDelta {
			t.Errorf("breakout not sorted: %d < %d", bo[i-1].StarDelta, bo[i].StarDelta)
		}
	}

	// I5b: trending filter
	for _, r := range tr {
		if r.StartStars < 100 {
			t.Errorf("trending repo %d start < 100: %d", r.RepoID, r.StartStars)
		}
		if r.GrowthPct <= 0 {
			t.Errorf("trending repo %d growth <= 0: %v", r.RepoID, r.GrowthPct)
		}
	}
	for i := 1; i < len(tr); i++ {
		if tr[i-1].GrowthPct < tr[i].GrowthPct {
			t.Errorf("trending not sorted: %v < %v", tr[i-1].GrowthPct, tr[i].GrowthPct)
		}
	}

	// I5d: no overlap
	in := map[int64]bool{}
	for _, r := range bo {
		in[r.RepoID] = true
	}
	for _, r := range tr {
		if in[r.RepoID] {
			t.Errorf("repo %d in both axes (1d)", r.RepoID)
		}
	}
}
