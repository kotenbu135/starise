// Package pipeline is the orchestration glue between db, ranking, and export.
// Testable functions with no side effects beyond the DB handle they receive.
package pipeline

import (
	"database/sql"
	"fmt"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// AllPeriods is the set of ranking windows computed by Compute.
var AllPeriods = []ranking.Period{ranking.Period1d, ranking.Period7d, ranking.Period30d}

// Compute derives rankings from the DB state for all periods anchored at
// computedDate, validates invariants, and writes results to the rankings
// table. Returns an error (to be treated as fatal by the CLI) on any
// invariant violation or DB error.
func Compute(database *sql.DB, computedDate string) error {
	repos, err := db.ListRepositories(database)
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	byPeriod := make(map[ranking.Period][]ranking.RepoGrowth, len(AllPeriods))
	for _, period := range AllPeriods {
		rows, err := computeForPeriod(database, repos, computedDate, period)
		if err != nil {
			return fmt.Errorf("compute %s: %w", period, err)
		}
		byPeriod[period] = rows
	}

	// Macro check: at least one period must have produced rows. Catches
	// silent pipeline failures (e.g. all repos excluded, no star history).
	if err := ranking.MacroValidate(byPeriod); err != nil {
		return err
	}

	for _, period := range AllPeriods {
		if err := PersistRanking(database, computedDate, period, byPeriod[period]); err != nil {
			return fmt.Errorf("persist %s: %w", period, err)
		}
	}
	return nil
}

// computeForPeriod produces ranked rows for one period across all repos.
// Repos excluded by ranking rules (insufficient history, below threshold)
// are simply absent from the output.
func computeForPeriod(database *sql.DB, repos []db.Repository, endDate string, period ranking.Period) ([]ranking.RepoGrowth, error) {
	var included []ranking.RepoGrowth
	for _, r := range repos {
		snaps, err := db.ListDailyStars(database, r.ID)
		if err != nil {
			return nil, fmt.Errorf("list stars repo=%d: %w", r.ID, err)
		}
		if len(snaps) == 0 {
			continue
		}
		s := toSnapshots(snaps)
		g, ok := ranking.ComputeRepoGrowth(s, endDate, period)
		if !ok {
			continue
		}
		g.RepoID = r.ID
		g.Period = period
		included = append(included, g)
	}
	return ranking.AssignRanks(included), nil
}

// PersistRanking validates invariants and replaces the (period, computedDate)
// slice in the rankings table. Exposed so tests can exercise the validation
// path independently.
func PersistRanking(database *sql.DB, computedDate string, period ranking.Period, rows []ranking.RepoGrowth) error {
	if err := ranking.Validate(rows); err != nil {
		return fmt.Errorf("invariants %s: %w", period, err)
	}
	dbRows := make([]db.Ranking, 0, len(rows))
	for _, r := range rows {
		dbRows = append(dbRows, db.Ranking{
			RepoID:       r.RepoID,
			Period:       string(period),
			ComputedDate: computedDate,
			StartStars:   r.StartStars,
			EndStars:     r.EndStars,
			StarDelta:    r.StarDelta,
			GrowthPct:    r.GrowthPct,
			Rank:         r.Rank,
		})
	}
	return db.ReplaceRankingsForDate(database, string(period), computedDate, dbRows)
}

func toSnapshots(stars []db.DailyStar) []ranking.Snapshot {
	out := make([]ranking.Snapshot, 0, len(stars))
	for _, s := range stars {
		out = append(out, ranking.Snapshot{Date: s.RecordedDate, Stars: s.StarCount})
	}
	return out
}
