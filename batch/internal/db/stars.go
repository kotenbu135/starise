package db

import (
	"database/sql"
	"fmt"
)

// DailyStar is one snapshot of a repo's star count on a given date.
type DailyStar struct {
	ID           int64
	RepoID       int64
	RecordedDate string // YYYY-MM-DD
	StarCount    int
}

// UpsertDailyStar writes (or overwrites) the star snapshot for repo_id on
// recorded_date.
func UpsertDailyStar(d *sql.DB, s *DailyStar) error {
	const q = `
INSERT INTO daily_stars (repo_id, recorded_date, star_count)
VALUES (?, ?, ?)
ON CONFLICT (repo_id, recorded_date) DO UPDATE SET
	star_count = excluded.star_count;
`
	if _, err := d.Exec(q, s.RepoID, s.RecordedDate, s.StarCount); err != nil {
		return fmt.Errorf("upsert daily_star repo=%d date=%s: %w", s.RepoID, s.RecordedDate, err)
	}
	return nil
}

// GetDailyStar returns the snapshot for a given repo+date or ErrNotFound.
func GetDailyStar(d *sql.DB, repoID int64, date string) (*DailyStar, error) {
	const q = `SELECT id, repo_id, recorded_date, star_count FROM daily_stars
		WHERE repo_id = ? AND recorded_date = ?;`
	var s DailyStar
	err := d.QueryRow(q, repoID, date).Scan(&s.ID, &s.RepoID, &s.RecordedDate, &s.StarCount)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get daily_star: %w", err)
	}
	return &s, nil
}

// GetStarAtOrBefore returns the most recent snapshot whose recorded_date <= target.
// Used by ranking to find the starting stars for a period window.
func GetStarAtOrBefore(d *sql.DB, repoID int64, target string) (*DailyStar, error) {
	const q = `SELECT id, repo_id, recorded_date, star_count FROM daily_stars
		WHERE repo_id = ? AND recorded_date <= ?
		ORDER BY recorded_date DESC LIMIT 1;`
	var s DailyStar
	err := d.QueryRow(q, repoID, target).Scan(&s.ID, &s.RepoID, &s.RecordedDate, &s.StarCount)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get star at or before: %w", err)
	}
	return &s, nil
}

// ListDailyStars returns all snapshots for a repo in ascending date order.
func ListDailyStars(d *sql.DB, repoID int64) ([]DailyStar, error) {
	const q = `SELECT id, repo_id, recorded_date, star_count FROM daily_stars
		WHERE repo_id = ? ORDER BY recorded_date ASC;`
	rows, err := d.Query(q, repoID)
	if err != nil {
		return nil, fmt.Errorf("list daily_stars: %w", err)
	}
	defer rows.Close()

	var out []DailyStar
	for rows.Next() {
		var s DailyStar
		if err := rows.Scan(&s.ID, &s.RepoID, &s.RecordedDate, &s.StarCount); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
