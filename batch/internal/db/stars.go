package db

import (
	"database/sql"
	"fmt"
)

type DailyStar struct {
	RepoID       int64
	RecordedDate string
	StarCount    int
}

func UpsertDailyStar(db *sql.DB, s *DailyStar) error {
	_, err := db.Exec(`
		INSERT INTO daily_stars (repo_id, recorded_date, star_count)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_id, recorded_date) DO UPDATE SET star_count=excluded.star_count`,
		s.RepoID, s.RecordedDate, s.StarCount,
	)
	if err != nil {
		return fmt.Errorf("upsert daily star: %w", err)
	}
	return nil
}

type StarPair struct {
	RepoID    int64
	StarStart int
	StarEnd   int
}

// GetStarPairs returns (start, end) pairs for repos with an exact N-day-old snapshot.
// Repos lacking a historical row for the requested period are excluded — mixing them
// in would require a StarStart=0 fallback, which makes growth_rate collapse to the
// absolute star count and corrupts the ranking.
func GetStarPairs(db *sql.DB, days int) ([]StarPair, error) {
	rows, err := db.Query(`
		SELECT s_end.repo_id, s_start.star_count, s_end.star_count
		FROM daily_stars s_end
		INNER JOIN daily_stars s_start
			ON s_start.repo_id = s_end.repo_id
			AND s_start.recorded_date = date(s_end.recorded_date, ?)
		WHERE s_end.recorded_date = (SELECT MAX(recorded_date) FROM daily_stars)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return nil, fmt.Errorf("get star pairs: %w", err)
	}
	defer rows.Close()

	var pairs []StarPair
	for rows.Next() {
		var p StarPair
		if err := rows.Scan(&p.RepoID, &p.StarStart, &p.StarEnd); err != nil {
			return nil, fmt.Errorf("scan star pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

type StarHistory struct {
	Date  string
	Stars int
}

func GetStarHistory(db *sql.DB, repoID int64) ([]StarHistory, error) {
	rows, err := db.Query(`
		SELECT recorded_date, star_count FROM daily_stars
		WHERE repo_id = ? ORDER BY recorded_date`, repoID)
	if err != nil {
		return nil, fmt.Errorf("get star history: %w", err)
	}
	defer rows.Close()

	var history []StarHistory
	for rows.Next() {
		var h StarHistory
		if err := rows.Scan(&h.Date, &h.Stars); err != nil {
			return nil, fmt.Errorf("scan star history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

func UpsertRanking(db *sql.DB, repoID int64, period, computedDate string, starStart, starEnd, starDelta int, growthRate float64, rank int) error {
	_, err := db.Exec(`
		INSERT INTO rankings (repo_id, period, computed_date, star_start, star_end, star_delta, growth_rate, rank)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, period, computed_date) DO UPDATE SET
			star_start=excluded.star_start, star_end=excluded.star_end,
			star_delta=excluded.star_delta, growth_rate=excluded.growth_rate, rank=excluded.rank`,
		repoID, period, computedDate, starStart, starEnd, starDelta, growthRate, rank,
	)
	if err != nil {
		return fmt.Errorf("upsert ranking: %w", err)
	}
	return nil
}
