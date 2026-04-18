package pipeline

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/ranking"
)

// I12: when all 6 ranking slots end up empty, the pipeline check returns
// an error so callers can exit non-zero.
func TestInvariantI12_EmptyRankingsAborts_Real(t *testing.T) {
	d := openMem(t)
	// no repos, no history → all slots empty
	err := ranking.ComputeAndCheck(d, "2026-04-18", 100)
	if err == nil {
		t.Fatalf("expected error from ComputeAndCheck on empty pipeline")
	}
}

// Sanity: when at least one slot is populated, ComputeAndCheck succeeds.
func TestInvariantI12_NonEmptyAccepts(t *testing.T) {
	d := openMem(t)
	mustUpsert(t, d, "A", "x", "a", false, false, map[string]int{"2026-04-17": 5, "2026-04-18": 100})
	if err := ranking.ComputeAndCheck(d, "2026-04-18", 100); err != nil {
		t.Errorf("non-empty pipeline rejected: %v", err)
	}
}
