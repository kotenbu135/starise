package db

import (
	"testing"
)

func TestMigrateCreatesAllTables(t *testing.T) {
	d := openMemory(t)

	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	tables := []string{"repositories", "daily_stars", "rankings"}
	for _, tbl := range tables {
		var name string
		err := d.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", tbl, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	d := openMemory(t)

	for i := 0; i < 3; i++ {
		if err := Migrate(d); err != nil {
			t.Fatalf("Migrate #%d: %v", i, err)
		}
	}
}

func TestRepositoriesUniqueOwnerName(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	_, err := d.Exec(
		`INSERT INTO repositories (github_id, owner, name) VALUES (?, ?, ?)`,
		"g1", "owner", "repo",
	)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, err = d.Exec(
		`INSERT INTO repositories (github_id, owner, name) VALUES (?, ?, ?)`,
		"g2", "owner", "repo",
	)
	if err == nil {
		t.Fatal("expected UNIQUE violation on (owner,name), got nil")
	}
}

func TestDailyStarsUniqueRepoDate(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	_, err := d.Exec(
		`INSERT INTO repositories (github_id, owner, name) VALUES (?, ?, ?)`,
		"g1", "o", "r",
	)
	if err != nil {
		t.Fatalf("repo insert: %v", err)
	}

	_, err = d.Exec(
		`INSERT INTO daily_stars (repo_id, recorded_date, star_count) VALUES (1, '2026-04-18', 100)`,
	)
	if err != nil {
		t.Fatalf("first star: %v", err)
	}
	_, err = d.Exec(
		`INSERT INTO daily_stars (repo_id, recorded_date, star_count) VALUES (1, '2026-04-18', 200)`,
	)
	if err == nil {
		t.Fatal("expected UNIQUE violation on (repo_id,recorded_date)")
	}
}

func TestRankingsUniqueRepoPeriodDate(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	_, err := d.Exec(
		`INSERT INTO repositories (github_id, owner, name) VALUES (?, ?, ?)`,
		"g1", "o", "r",
	)
	if err != nil {
		t.Fatalf("repo insert: %v", err)
	}

	_, err = d.Exec(
		`INSERT INTO rankings (repo_id, period, computed_date, start_stars, end_stars, star_delta, growth_pct, rank)
		 VALUES (1, '7d', '2026-04-18', 100, 150, 50, 50.0, 1)`,
	)
	if err != nil {
		t.Fatalf("first ranking: %v", err)
	}
	_, err = d.Exec(
		`INSERT INTO rankings (repo_id, period, computed_date, start_stars, end_stars, star_delta, growth_pct, rank)
		 VALUES (1, '7d', '2026-04-18', 100, 160, 60, 60.0, 2)`,
	)
	if err == nil {
		t.Fatal("expected UNIQUE violation on (repo_id,period,computed_date)")
	}
}
