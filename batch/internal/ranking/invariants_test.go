package ranking

import (
	"math"
	"strings"
	"testing"
)

func TestInvariantsPassOnHealthySet(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, StartStars: 100, EndStars: 150, StarDelta: 50, GrowthPct: 50.0, Rank: 1},
		{RepoID: 2, Period: Period7d, StartStars: 200, EndStars: 240, StarDelta: 40, GrowthPct: 20.0, Rank: 2},
		{RepoID: 3, Period: Period7d, StartStars: 300, EndStars: 330, StarDelta: 30, GrowthPct: 10.0, Rank: 3},
	}
	if err := Validate(rows); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestInvariantsAllowEmptyPeriod(t *testing.T) {
	// Empty is legitimate for a single period (macro-level emptiness is
	// checked by the caller across all periods).
	if err := Validate(nil); err != nil {
		t.Errorf("expected nil for empty, got %v", err)
	}
}

func TestMacroValidateRejectsAllEmpty(t *testing.T) {
	err := MacroValidate(map[Period][]RepoGrowth{
		Period1d:  nil,
		Period7d:  nil,
		Period30d: nil,
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected macro empty error, got %v", err)
	}
}

func TestMacroValidatePassesIfOneNonEmpty(t *testing.T) {
	err := MacroValidate(map[Period][]RepoGrowth{
		Period1d:  nil,
		Period7d:  {{RepoID: 1, StartStars: 10, EndStars: 20, StarDelta: 10, GrowthPct: 100, Rank: 1}},
		Period30d: nil,
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestInvariantsRejectsNaN(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: math.NaN(), Rank: 1, StartStars: 10, EndStars: 20, StarDelta: 10},
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "NaN") {
		t.Errorf("expected NaN error, got %v", err)
	}
}

func TestInvariantsRejectsInf(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: math.Inf(1), Rank: 1, StartStars: 10, EndStars: 20, StarDelta: 10},
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "Inf") {
		t.Errorf("expected Inf error, got %v", err)
	}
}

func TestInvariantsRejectsNonMonotonicRank(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: 50.0, Rank: 1, StartStars: 10, EndStars: 15, StarDelta: 5},
		{RepoID: 2, Period: Period7d, GrowthPct: 70.0, Rank: 2, StartStars: 10, EndStars: 17, StarDelta: 7}, // higher pct at lower rank → invalid
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "rank") {
		t.Errorf("expected rank ordering error, got %v", err)
	}
}

func TestInvariantsRejectsDuplicateRank(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: 50.0, Rank: 1, StartStars: 10, EndStars: 15, StarDelta: 5},
		{RepoID: 2, Period: Period7d, GrowthPct: 40.0, Rank: 1, StartStars: 10, EndStars: 14, StarDelta: 4},
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate rank error, got %v", err)
	}
}

func TestInvariantsRejectsBadStartStars(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: 50.0, Rank: 1, StartStars: 0, EndStars: 50},
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "start_stars") {
		t.Errorf("expected start_stars error, got %v", err)
	}
}

func TestInvariantsRejectsInconsistentDelta(t *testing.T) {
	rows := []RepoGrowth{
		{RepoID: 1, Period: Period7d, GrowthPct: 50.0, Rank: 1,
			StartStars: 100, EndStars: 150, StarDelta: 999 /* should be 50 */},
	}
	err := Validate(rows)
	if err == nil || !strings.Contains(err.Error(), "delta") {
		t.Errorf("expected delta inconsistency error, got %v", err)
	}
}
