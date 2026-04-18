package db

import (
	"database/sql"
	"errors"
)

type DailyStar struct {
	RepoID       int64
	RecordedDate string
	StarCount    int
}

// UpsertDailyStar overwrites the row for (repo_id, recorded_date).
func UpsertDailyStar(d *sql.DB, repoID int64, date string, count int) error {
	const q = `INSERT INTO daily_stars (repo_id, recorded_date, star_count)
        VALUES (?, ?, ?)
        ON CONFLICT(repo_id, recorded_date) DO UPDATE SET star_count=excluded.star_count`
	_, err := d.Exec(q, repoID, date, count)
	return err
}

func ListStarHistory(d *sql.DB, repoID int64) ([]DailyStar, error) {
	rows, err := d.Query(`SELECT repo_id, recorded_date, star_count FROM daily_stars
        WHERE repo_id=? ORDER BY recorded_date`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyStar
	for rows.Next() {
		var s DailyStar
		if err := rows.Scan(&s.RepoID, &s.RecordedDate, &s.StarCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// StarCountAtOrBefore returns the most recent star_count on or before the given date.
// If the date is later than the latest record, returns the latest record.
// If no record exists at all on or before the date, ok=false.
func StarCountAtOrBefore(d *sql.DB, repoID int64, date string) (int, bool, error) {
	var n int
	err := d.QueryRow(`SELECT star_count FROM daily_stars
        WHERE repo_id=? AND recorded_date <= ?
        ORDER BY recorded_date DESC LIMIT 1`, repoID, date).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		// Fall back to the earliest record only when the date is BEFORE the earliest.
		// In that case the function contract says ok=false; we just return.
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}
