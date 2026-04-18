package db

import (
	"database/sql"
	"fmt"
)

// Ranking is one row of the rankings table.
type Ranking struct {
	ID           int64
	RepoID       int64
	Period       string // "1d", "7d", "30d"
	ComputedDate string // YYYY-MM-DD
	StartStars   int
	EndStars     int
	StarDelta    int
	GrowthPct    float64
	Rank         int
}

// ReplaceRankingsForDate atomically replaces all rankings matching
// (period, computed_date) with the provided set.
func ReplaceRankingsForDate(d *sql.DB, period, date string, rows []Ranking) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM rankings WHERE period = ? AND computed_date = ?;`,
		period, date,
	); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}

	const q = `
INSERT INTO rankings (repo_id, period, computed_date, start_stars, end_stars, star_delta, growth_pct, rank)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`
	stmt, err := tx.Prepare(q)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.Exec(
			r.RepoID, r.Period, r.ComputedDate,
			r.StartStars, r.EndStars, r.StarDelta, r.GrowthPct, r.Rank,
		); err != nil {
			return fmt.Errorf("insert ranking repo=%d rank=%d: %w", r.RepoID, r.Rank, err)
		}
	}

	return tx.Commit()
}

// ListRankings returns rankings for (period, date) in ascending rank order.
// limit <= 0 returns all rows.
func ListRankings(d *sql.DB, period, date string, limit int) ([]Ranking, error) {
	q := `
SELECT id, repo_id, period, computed_date, start_stars, end_stars, star_delta, growth_pct, rank
FROM rankings WHERE period = ? AND computed_date = ?
ORDER BY rank ASC`
	args := []any{period, date}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	q += ";"

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list rankings: %w", err)
	}
	defer rows.Close()

	var out []Ranking
	for rows.Next() {
		var r Ranking
		if err := rows.Scan(
			&r.ID, &r.RepoID, &r.Period, &r.ComputedDate,
			&r.StartStars, &r.EndStars, &r.StarDelta, &r.GrowthPct, &r.Rank,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
