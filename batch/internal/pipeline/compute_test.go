package pipeline

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	_ "modernc.org/sqlite"
)

func TestComputeWritesAllPeriods(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	repo, _ := db.UpsertRepository(database, &db.Repository{GitHubID: "g1", Owner: "o", Name: "r"})
	for _, s := range []db.DailyStar{
		{RepoID: repo, RecordedDate: "2026-03-19", StarCount: 50},
		{RepoID: repo, RecordedDate: "2026-04-11", StarCount: 100},
		{RepoID: repo, RecordedDate: "2026-04-17", StarCount: 150},
		{RepoID: repo, RecordedDate: "2026-04-18", StarCount: 160},
	} {
		_ = db.UpsertDailyStar(database, &s)
	}

	if err := Compute(database, "2026-04-18"); err != nil {
		t.Fatalf("compute: %v", err)
	}

	for _, p := range []string{"1d", "7d", "30d"} {
		rows, err := db.ListRankings(database, p, "2026-04-18", 100)
		if err != nil {
			t.Fatalf("list %s: %v", p, err)
		}
		if len(rows) != 1 {
			t.Errorf("period %s rows=%d want 1", p, len(rows))
		}
	}
}

func TestComputeExcludesBelowThreshold(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	// Repo A: valid.
	a, _ := db.UpsertRepository(database, &db.Repository{GitHubID: "ga", Owner: "o", Name: "a"})
	_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: a, RecordedDate: "2026-04-11", StarCount: 100})
	_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: a, RecordedDate: "2026-04-18", StarCount: 150})

	// Repo B: start below threshold → excluded for 7d.
	b, _ := db.UpsertRepository(database, &db.Repository{GitHubID: "gb", Owner: "o", Name: "b"})
	_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: b, RecordedDate: "2026-04-11", StarCount: 5})
	_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: b, RecordedDate: "2026-04-18", StarCount: 500})

	if err := Compute(database, "2026-04-18"); err != nil {
		t.Fatalf("compute: %v", err)
	}

	rows, _ := db.ListRankings(database, "7d", "2026-04-18", 100)
	if len(rows) != 1 {
		t.Fatalf("7d rows=%d want 1", len(rows))
	}
	if rows[0].RepoID != a {
		t.Errorf("expected repo A (threshold-safe), got repo %d", rows[0].RepoID)
	}
}

func TestComputeRanksAreSequential(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	for i := 1; i <= 5; i++ {
		repo, _ := db.UpsertRepository(database, &db.Repository{
			GitHubID: "g" + string(rune('0'+i)), Owner: "o", Name: "r" + string(rune('0'+i)),
		})
		_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: repo, RecordedDate: "2026-04-11", StarCount: 100})
		_ = db.UpsertDailyStar(database, &db.DailyStar{RepoID: repo, RecordedDate: "2026-04-18", StarCount: 100 + i*10})
	}
	if err := Compute(database, "2026-04-18"); err != nil {
		t.Fatalf("compute: %v", err)
	}

	rows, _ := db.ListRankings(database, "7d", "2026-04-18", 100)
	if len(rows) != 5 {
		t.Fatalf("rows=%d want 5", len(rows))
	}
	for i, r := range rows {
		if r.Rank != i+1 {
			t.Errorf("row %d rank=%d want %d", i, r.Rank, i+1)
		}
	}
}

func TestComputeInvariantFailureReturnsError(t *testing.T) {
	// Build a set that violates invariants by short-circuiting to a poisoned
	// slice. Use the lower-level write path.
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	repo, _ := db.UpsertRepository(database, &db.Repository{GitHubID: "g", Owner: "o", Name: "r"})

	// Bad: start_stars=0 violates invariant.
	rows := []ranking.RepoGrowth{
		{RepoID: repo, Period: ranking.Period7d, StartStars: 0, EndStars: 50, StarDelta: 50, GrowthPct: 50, Rank: 1},
	}
	err := PersistRanking(database, "2026-04-18", ranking.Period7d, rows)
	if err == nil {
		t.Fatal("expected invariant error")
	}
}
