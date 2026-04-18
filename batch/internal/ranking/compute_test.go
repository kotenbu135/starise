package ranking

import (
	"database/sql"
	"math"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func seed(t *testing.T, d *sql.DB, owner, name, startDate string, startStars int, endDate string, endStars int) int64 {
	t.Helper()
	id, err := db.UpsertRepository(d, &db.Repository{
		GitHubID: owner + "/" + name, Owner: owner, Name: name,
	})
	if err != nil {
		t.Fatalf("seed repo: %v", err)
	}
	if startDate != "" {
		if err := db.UpsertDailyStar(d, &db.DailyStar{RepoID: id, RecordedDate: startDate, StarCount: startStars}); err != nil {
			t.Fatalf("seed start star: %v", err)
		}
	}
	if err := db.UpsertDailyStar(d, &db.DailyStar{RepoID: id, RecordedDate: endDate, StarCount: endStars}); err != nil {
		t.Fatalf("seed end star: %v", err)
	}
	return id
}

func fetchRankings(t *testing.T, d *sql.DB, period string) []rankRow {
	t.Helper()
	rows, err := d.Query(`
		SELECT r.owner, r.name, rk.star_start, rk.star_end, rk.star_delta, rk.growth_rate, rk.rank
		FROM rankings rk JOIN repositories r ON r.id = rk.repo_id
		WHERE rk.period = ? ORDER BY rk.rank`, period)
	if err != nil {
		t.Fatalf("query rankings: %v", err)
	}
	defer rows.Close()
	var out []rankRow
	for rows.Next() {
		var r rankRow
		if err := rows.Scan(&r.Owner, &r.Name, &r.Start, &r.End, &r.Delta, &r.Rate, &r.Rank); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

type rankRow struct {
	Owner, Name    string
	Start, End     int
	Delta, Rank    int
	Rate           float64
}

func TestCompute_GrowthRateMath(t *testing.T) {
	d := newTestDB(t)
	// 7-day window: end=2026-04-17 (MAX), start=2026-04-10
	seed(t, d, "acme", "fast", "2026-04-10", 100, "2026-04-17", 200)   // +100%
	seed(t, d, "acme", "slow", "2026-04-10", 100, "2026-04-17", 110)   // +10%
	seed(t, d, "acme", "flat", "2026-04-10", 100, "2026-04-17", 100)   // 0%
	seed(t, d, "acme", "drop", "2026-04-10", 100, "2026-04-17", 50)    // -50%

	if err := Compute(d); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	rows := fetchRankings(t, d, "7d")
	if len(rows) != 4 {
		t.Fatalf("got %d rankings, want 4", len(rows))
	}

	wantByName := map[string]float64{
		"fast": 100, "slow": 10, "flat": 0, "drop": -50,
	}
	for _, r := range rows {
		want := wantByName[r.Name]
		if math.Abs(r.Rate-want) > 0.01 {
			t.Errorf("%s: rate=%.2f, want %.2f", r.Name, r.Rate, want)
		}
	}

	// Rank order: fast(1), slow(2), flat(3), drop(4)
	order := []string{"fast", "slow", "flat", "drop"}
	for i, r := range rows {
		if r.Name != order[i] {
			t.Errorf("rank %d: got %s, want %s", r.Rank, r.Name, order[i])
		}
		if r.Rank != i+1 {
			t.Errorf("%s: rank=%d, want %d", r.Name, r.Rank, i+1)
		}
	}
}

func TestCompute_SkipsRepoWithoutHistory(t *testing.T) {
	d := newTestDB(t)
	// Has 7-day-old row → should rank
	seed(t, d, "acme", "tracked", "2026-04-10", 50, "2026-04-17", 100)
	// No 7-day-old row → previously caused StarStart=0 bug, now excluded
	seed(t, d, "acme", "orphan", "", 0, "2026-04-17", 99999)

	if err := Compute(d); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	rows := fetchRankings(t, d, "7d")
	if len(rows) != 1 {
		t.Fatalf("got %d rankings, want 1 (orphan must be skipped): %+v", len(rows), rows)
	}
	if rows[0].Name != "tracked" {
		t.Errorf("got %s, want tracked", rows[0].Name)
	}
	// Bug A fingerprint: rate == delta == end must not appear for real entries
	if rows[0].Rate == float64(rows[0].Delta) && rows[0].Delta == rows[0].End {
		t.Errorf("Bug A regression: rate(%f) == delta(%d) == end(%d)", rows[0].Rate, rows[0].Delta, rows[0].End)
	}
}

func TestCompute_AllPeriodsIndependent(t *testing.T) {
	d := newTestDB(t)
	id, err := db.UpsertRepository(d, &db.Repository{GitHubID: "acme/multi", Owner: "acme", Name: "multi"})
	if err != nil {
		t.Fatal(err)
	}
	// Feed 30d, 7d, 1d baselines — all relative to MAX(2026-04-17)=100
	for _, s := range []struct {
		date  string
		count int
	}{
		{"2026-03-18", 10}, // 30 days before
		{"2026-04-10", 50}, // 7 days before
		{"2026-04-16", 80}, // 1 day before
		{"2026-04-17", 100},
	} {
		if err := db.UpsertDailyStar(d, &db.DailyStar{RepoID: id, RecordedDate: s.date, StarCount: s.count}); err != nil {
			t.Fatal(err)
		}
	}

	if err := Compute(d); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	want := map[string]float64{
		"1d":  25,  // (100-80)/80 * 100
		"7d":  100, // (100-50)/50 * 100
		"30d": 900, // (100-10)/10 * 100
	}
	for period, wantRate := range want {
		rows := fetchRankings(t, d, period)
		if len(rows) != 1 {
			t.Errorf("%s: got %d rankings, want 1", period, len(rows))
			continue
		}
		if math.Abs(rows[0].Rate-wantRate) > 0.01 {
			t.Errorf("%s: rate=%.2f, want %.2f", period, rows[0].Rate, wantRate)
		}
	}
}

func TestCompute_IsIdempotent(t *testing.T) {
	d := newTestDB(t)
	seed(t, d, "acme", "alpha", "2026-04-10", 100, "2026-04-17", 150)

	for i := 0; i < 3; i++ {
		if err := Compute(d); err != nil {
			t.Fatalf("Compute iter %d: %v", i, err)
		}
	}
	rows := fetchRankings(t, d, "7d")
	if len(rows) != 1 {
		t.Fatalf("got %d, want 1 (UPSERT should dedupe)", len(rows))
	}
}
