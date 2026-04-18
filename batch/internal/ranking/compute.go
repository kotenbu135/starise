package ranking

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
)

type Entry struct {
	RepoID    int64
	StarStart int
	StarEnd   int
	StarDelta int
	GrowthRate float64
}

func Compute(database *sql.DB) error {
	today := time.Now().UTC().Format("2006-01-02")

	for _, period := range []struct {
		Name string
		Days int
	}{
		{"1d", 1},
		{"7d", 7},
		{"30d", 30},
	} {
		pairs, err := db.GetStarPairs(database, period.Days)
		if err != nil {
			return fmt.Errorf("get star pairs (%s): %w", period.Name, err)
		}

		entries := make([]Entry, 0, len(pairs))
		for _, p := range pairs {
			if p.StarStart <= 0 {
				continue
			}
			delta := p.StarEnd - p.StarStart
			rate := float64(delta) / float64(p.StarStart) * 100
			entries = append(entries, Entry{
				RepoID:     p.RepoID,
				StarStart:  p.StarStart,
				StarEnd:    p.StarEnd,
				StarDelta:  delta,
				GrowthRate: rate,
			})
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].GrowthRate > entries[j].GrowthRate
		})

		// One transaction per period — 38k+ upserts go from ~12s → ~0.5s.
		tx, err := database.Begin()
		if err != nil {
			return fmt.Errorf("begin tx (%s): %w", period.Name, err)
		}
		stmt, err := tx.Prepare(`
			INSERT INTO rankings (repo_id, period, computed_date, star_start, star_end, star_delta, growth_rate, rank)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(repo_id, period, computed_date) DO UPDATE SET
				star_start=excluded.star_start, star_end=excluded.star_end,
				star_delta=excluded.star_delta, growth_rate=excluded.growth_rate, rank=excluded.rank`)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("prepare (%s): %w", period.Name, err)
		}
		for rank, e := range entries {
			if _, err := stmt.Exec(e.RepoID, period.Name, today,
				e.StarStart, e.StarEnd, e.StarDelta, e.GrowthRate, rank+1); err != nil {
				_ = stmt.Close()
				_ = tx.Rollback()
				return fmt.Errorf("upsert ranking: %w", err)
			}
		}
		if err := stmt.Close(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("close stmt (%s): %w", period.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit (%s): %w", period.Name, err)
		}

		log.Printf("Computed %s rankings: %d entries", period.Name, len(entries))
	}
	return nil
}
