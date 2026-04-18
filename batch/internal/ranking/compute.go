package ranking

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
)

// periodToDays maps the period label to its lookback window in days.
var periodToDays = map[string]int{
	"1d":  1,
	"7d":  7,
	"30d": 30,
}

// Compute calculates and writes all 6 ranking slots for the given computedDate.
// Source rows are non-archived, non-deleted, non-fork repositories with a
// star snapshot at computedDate AND on/before computedDate-N days.
// topN caps each slot independently.
func Compute(d *sql.DB, computedDate string, topN int) error {
	if topN <= 0 {
		return fmt.Errorf("topN must be positive, got %d", topN)
	}
	end, err := time.Parse("2006-01-02", computedDate)
	if err != nil {
		return fmt.Errorf("parse computedDate: %w", err)
	}

	repos, err := db.ListActiveRepositories(d)
	if err != nil {
		return fmt.Errorf("list active: %w", err)
	}
	// Drop forks — listActive already drops archived + deleted.
	active := make([]db.Repository, 0, len(repos))
	for _, r := range repos {
		if r.IsFork {
			continue
		}
		active = append(active, r)
	}

	for _, period := range Periods {
		days := periodToDays[period]
		startDate := end.AddDate(0, 0, -days).Format("2006-01-02")

		candidates := make([]Candidate, 0, len(active))
		for _, r := range active {
			endStars, hasEnd, err := db.StarCountAtOrBefore(d, r.ID, computedDate)
			if err != nil {
				return fmt.Errorf("end snapshot: %w", err)
			}
			if !hasEnd {
				continue
			}
			startStars, hasStart, err := db.StarCountAtOrBefore(d, r.ID, startDate)
			if err != nil {
				return fmt.Errorf("start snapshot: %w", err)
			}
			if !hasStart {
				// Repo first appeared after the period start: use the earliest
				// known snapshot as the lookback baseline so newly-discovered
				// breakouts are still surfaced. EarliestStarCount returns at
				// least the same row that produced endStars, so it cannot
				// invent missing data.
				startStars, hasStart, err = db.EarliestStarCount(d, r.ID)
				if err != nil {
					return fmt.Errorf("earliest start snapshot: %w", err)
				}
				if !hasStart {
					continue
				}
			}
			candidates = append(candidates, Candidate{
				RepoID:     r.ID,
				StartStars: startStars,
				EndStars:   endStars,
			})
		}

		breakout := capN(ComputeBreakout(candidates), topN)
		trending := capN(ComputeTrending(candidates), topN)

		if err := Validate(breakout, RankTypeBreakout); err != nil {
			return fmt.Errorf("validate %s/breakout: %w", period, err)
		}
		if err := Validate(trending, RankTypeTrending); err != nil {
			return fmt.Errorf("validate %s/trending: %w", period, err)
		}
		if err := ValidateNoOverlap(breakout, trending); err != nil {
			return fmt.Errorf("validate %s overlap: %w", period, err)
		}

		if err := writeSlot(d, period, RankTypeBreakout, computedDate, breakout); err != nil {
			return err
		}
		if err := writeSlot(d, period, RankTypeTrending, computedDate, trending); err != nil {
			return err
		}
	}
	return nil
}

// ComputeAndCheck runs Compute and additionally enforces MacroValidate (I12).
// Returns an error when all 6 slots are empty so callers can exit non-zero.
func ComputeAndCheck(d *sql.DB, computedDate string, topN int) error {
	if err := Compute(d, computedDate, topN); err != nil {
		return err
	}
	all := map[string][]Scored{}
	for _, period := range Periods {
		for _, rt := range RankTypes {
			rows, err := db.ListRankings(d, period, rt, computedDate)
			if err != nil {
				return err
			}
			scored := make([]Scored, len(rows))
			for i, r := range rows {
				scored[i] = Scored{
					RepoID: r.RepoID, StartStars: r.StartStars, EndStars: r.EndStars,
					StarDelta: r.StarDelta, GrowthPct: r.GrowthPct, Rank: r.Rank,
				}
			}
			all[period+"_"+rt] = scored
		}
	}
	return MacroValidate(all)
}

func capN(rs []Scored, topN int) []Scored {
	if len(rs) > topN {
		return rs[:topN]
	}
	return rs
}

func writeSlot(d *sql.DB, period, rankType, computedDate string, rs []Scored) error {
	rows := make([]db.Ranking, len(rs))
	for i, r := range rs {
		rows[i] = db.Ranking{
			RepoID:       r.RepoID,
			Period:       period,
			RankType:     rankType,
			ComputedDate: computedDate,
			StartStars:   r.StartStars,
			EndStars:     r.EndStars,
			StarDelta:    r.StarDelta,
			GrowthPct:    r.GrowthPct,
			Rank:         r.Rank,
		}
	}
	if err := db.ReplaceRankings(d, period, rankType, computedDate, rows); err != nil {
		return fmt.Errorf("replace %s/%s: %w", period, rankType, err)
	}
	return nil
}
