package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestDB returns a fresh in-memory SQLite DB with schema applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedRepo(t *testing.T, db *sql.DB, owner, name string) int64 {
	t.Helper()
	id, err := UpsertRepository(db, &Repository{
		GitHubID: owner + "/" + name, Owner: owner, Name: name,
	})
	if err != nil {
		t.Fatalf("seed repo: %v", err)
	}
	return id
}

func seedStar(t *testing.T, db *sql.DB, repoID int64, date string, count int) {
	t.Helper()
	if err := UpsertDailyStar(db, &DailyStar{RepoID: repoID, RecordedDate: date, StarCount: count}); err != nil {
		t.Fatalf("seed star: %v", err)
	}
}

func TestGetStarPairs(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*testing.T, *sql.DB)
		days     int
		wantLen  int
		wantPair map[string][2]int // key: "owner/name" → [start, end]
	}{
		{
			name: "repo with both end and start date returns pair",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "alpha")
				seedStar(t, db, id, "2026-04-10", 80)
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:     7,
			wantLen:  1,
			wantPair: map[string][2]int{"acme/alpha": {80, 100}},
		},
		{
			name: "repo without historical row excluded",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "beta")
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:    7,
			wantLen: 0,
		},
		{
			name: "mixed: one with history, one without → only history included",
			setup: func(t *testing.T, db *sql.DB) {
				a := seedRepo(t, db, "acme", "alpha")
				seedStar(t, db, a, "2026-04-10", 80)
				seedStar(t, db, a, "2026-04-17", 100)
				b := seedRepo(t, db, "acme", "beta")
				seedStar(t, db, b, "2026-04-17", 500)
			},
			days:     7,
			wantLen:  1,
			wantPair: map[string][2]int{"acme/alpha": {80, 100}},
		},
		{
			name: "1d window uses date - 1",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "gamma")
				seedStar(t, db, id, "2026-04-16", 95)
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:     1,
			wantLen:  1,
			wantPair: map[string][2]int{"acme/gamma": {95, 100}},
		},
		{
			name: "30d window requires date - 30 row",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "delta")
				seedStar(t, db, id, "2026-03-18", 50)
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:     30,
			wantLen:  1,
			wantPair: map[string][2]int{"acme/delta": {50, 100}},
		},
		{
			name: "off-by-one date not matched",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "epsilon")
				seedStar(t, db, id, "2026-04-11", 80) // 6 days before, not 7
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:    7,
			wantLen: 0,
		},
		{
			name: "only end date present, no history at all",
			setup: func(t *testing.T, db *sql.DB) {
				id := seedRepo(t, db, "acme", "zeta")
				seedStar(t, db, id, "2026-04-17", 100)
			},
			days:    1,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			tt.setup(t, db)

			pairs, err := GetStarPairs(db, tt.days)
			if err != nil {
				t.Fatalf("GetStarPairs: %v", err)
			}
			if len(pairs) != tt.wantLen {
				t.Fatalf("len(pairs) = %d, want %d: %+v", len(pairs), tt.wantLen, pairs)
			}

			if tt.wantPair == nil {
				return
			}
			repos, err := GetAllRepositories(db)
			if err != nil {
				t.Fatalf("list repos: %v", err)
			}
			byID := map[int64]string{}
			for _, r := range repos {
				byID[r.ID] = r.Owner + "/" + r.Name
			}
			got := map[string][2]int{}
			for _, p := range pairs {
				got[byID[p.RepoID]] = [2]int{p.StarStart, p.StarEnd}
			}
			for k, want := range tt.wantPair {
				if g := got[k]; g != want {
					t.Errorf("pair[%s] = %v, want %v", k, g, want)
				}
			}
		})
	}
}

func TestGetStarPairs_MaxDateScoping(t *testing.T) {
	// GetStarPairs is anchored to MAX(recorded_date); older end_dates must not leak in.
	db := newTestDB(t)
	id := seedRepo(t, db, "acme", "alpha")
	seedStar(t, db, id, "2026-04-09", 70)  // would match as start for a 4/16 end
	seedStar(t, db, id, "2026-04-16", 90)  // old end, MUST NOT be selected as s_end
	seedStar(t, db, id, "2026-04-17", 100) // max
	// no 2026-04-10 row → 7d pair from 4/17 has no match

	pairs, err := GetStarPairs(db, 7)
	if err != nil {
		t.Fatalf("GetStarPairs: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("expected 0 pairs (no 4/10 row), got %+v", pairs)
	}
}
