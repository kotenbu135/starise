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
			delta := p.StarEnd - p.StarStart
			var rate float64
			if p.StarStart > 0 {
				rate = float64(delta) / float64(p.StarStart) * 100
			} else {
				rate = float64(delta)
			}
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

		for rank, e := range entries {
			if err := db.UpsertRanking(database, e.RepoID, period.Name, today,
				e.StarStart, e.StarEnd, e.StarDelta, e.GrowthRate, rank+1); err != nil {
				return fmt.Errorf("upsert ranking: %w", err)
			}
		}

		log.Printf("Computed %s rankings: %d entries", period.Name, len(entries))
	}
	return nil
}
