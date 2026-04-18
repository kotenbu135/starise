package db

import (
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	d := openMem(t)
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tables := []string{"repositories", "daily_stars", "rankings"}
	for _, tbl := range tables {
		var got string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&got)
		if err != nil {
			t.Errorf("table %s missing: %v", tbl, err)
		}
	}
}

// I10: Migrate idempotent.
func TestMigrateIdempotent(t *testing.T) {
	d := openMem(t)
	for i := 0; i < 5; i++ {
		if err := Migrate(d); err != nil {
			t.Fatalf("migrate iter %d: %v", i, err)
		}
	}
}

func TestMigrateCreatesRequiredColumns(t *testing.T) {
	d := openMem(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"repositories": {"github_id", "owner", "name", "deleted_at", "is_archived", "is_fork", "topics"},
		"daily_stars":  {"repo_id", "recorded_date", "star_count"},
		"rankings":     {"repo_id", "period", "rank_type", "computed_date", "start_stars", "end_stars", "star_delta", "growth_pct", "rank"},
	}
	for tbl, cols := range cases {
		rows, err := d.Query("PRAGMA table_info(" + tbl + ")")
		if err != nil {
			t.Fatalf("pragma %s: %v", tbl, err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt any
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				t.Fatal(err)
			}
			got[name] = true
		}
		rows.Close()
		for _, c := range cols {
			if !got[c] {
				t.Errorf("table %s missing column %s", tbl, c)
			}
		}
	}
}

func TestMigrateUniqueConstraints(t *testing.T) {
	d := openMem(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO repositories (github_id, owner, name) VALUES ('x', 'a', 'b')"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO repositories (github_id, owner, name) VALUES ('x', 'c', 'd')"); err == nil {
		t.Errorf("expected UNIQUE github_id violation")
	}
	if _, err := d.Exec("INSERT INTO repositories (github_id, owner, name) VALUES ('y', 'a', 'b')"); err == nil {
		t.Errorf("expected UNIQUE (owner,name) violation")
	}
}
