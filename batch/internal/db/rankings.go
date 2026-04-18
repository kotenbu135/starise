package db

import (
	"database/sql"
	"fmt"
)

type Ranking struct {
	RepoID       int64
	Period       string
	RankType     string
	ComputedDate string
	StartStars   int
	EndStars     int
	StarDelta    int
	GrowthPct    float64
	Rank         int
}

// ReplaceRankings clears all rows for (period, rank_type, computed_date) and inserts the new set.
func ReplaceRankings(d *sql.DB, period, rankType, computedDate string, rs []Ranking) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM rankings WHERE period=? AND rank_type=? AND computed_date=?`,
		period, rankType, computedDate); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	const ins = `INSERT INTO rankings
        (repo_id, period, rank_type, computed_date, start_stars, end_stars, star_delta, growth_pct, rank)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(ins)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rs {
		if _, err := stmt.Exec(r.RepoID, period, rankType, computedDate,
			r.StartStars, r.EndStars, r.StarDelta, r.GrowthPct, r.Rank); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}
	return tx.Commit()
}

func ListRankings(d *sql.DB, period, rankType, computedDate string) ([]Ranking, error) {
	rows, err := d.Query(`SELECT repo_id, period, rank_type, computed_date, start_stars, end_stars,
        star_delta, growth_pct, rank FROM rankings
        WHERE period=? AND rank_type=? AND computed_date=? ORDER BY rank`,
		period, rankType, computedDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ranking
	for rows.Next() {
		var r Ranking
		if err := rows.Scan(&r.RepoID, &r.Period, &r.RankType, &r.ComputedDate,
			&r.StartStars, &r.EndStars, &r.StarDelta, &r.GrowthPct, &r.Rank); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
