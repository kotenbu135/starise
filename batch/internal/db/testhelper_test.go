package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openMemory(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("fk: %v", err)
	}
	return d
}

func mustMigrate(t *testing.T, d *sql.DB) {
	t.Helper()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
}
