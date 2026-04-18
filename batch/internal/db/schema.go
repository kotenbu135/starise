package db

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

// Migrate applies the schema to the given database.
// Idempotent: safe to call multiple times against the same DB.
func Migrate(d *sql.DB) error {
	if _, err := d.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// Open opens a sqlite DB at path and enables foreign keys.
// An empty path is treated as a pure in-memory DB.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = ":memory:"
	}
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := d.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	return d, nil
}
