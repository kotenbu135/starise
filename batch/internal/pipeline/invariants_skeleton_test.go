// Package pipeline holds the end-to-end invariant tests for the v3 rewrite.
//
// Invariants I1-I13 are documented in GitHub issue #2 ("rewrite(batch) v3:
// invariant-driven TDD rewrite with 2-axis ranking"). Each test below is a
// RED placeholder until the corresponding package is implemented per the
// rewrite plan. Once an invariant has been promoted to its own
// invariant_iN_test.go file with real assertions, the matching Skip test
// here MUST be deleted.
//
// Done condition for the rewrite is: every test below is moved into a
// dedicated file with real assertions, all 13 GREEN, no skips remaining.
package pipeline

import "testing"

// I1: completeness — every non-deleted DB row has a matching repos/*.json file.
func TestInvariantI1_Completeness(t *testing.T) {
	t.Skip("pending: requires P13 export/schema, P14 export/json")
}

// I2: round-trip — DB -> export -> restore -> DB' has equal columns.
func TestInvariantI2_ExportRestoreRoundTrip(t *testing.T) {
	t.Skip("pending: requires P12 restore, P13 export/schema, P14 export/json")
}

// I3: 3-day simulation preserves Day-1 history through Day-3.
func TestInvariantI3_ThreeDaySimulation(t *testing.T) {
	t.Skip("pending: requires P17 pipeline/run, P18 pipeline/simulation")
}

// I4: refresh covers active repos today; >30% failure aborts pipeline.
func TestInvariantI4_RefreshFailureTolerance(t *testing.T) {
	t.Skip("pending: requires P11 refresh")
}

// I5: 2-axis ranking correctness (a: breakout, b: trending, c: no zero/neg, d: no overlap).
func TestInvariantI5_TwoAxisCorrectness(t *testing.T) {
	t.Skip("pending: requires P3 ranking/growth, P4 ranking/breakout, P5 ranking/compute")
}

// I6: no NaN, +Inf, -Inf in any ranking row.
func TestInvariantI6_NoNaNOrInf(t *testing.T) {
	t.Skip("pending: requires P5 ranking/compute, P6 ranking/invariants")
}

// I7: ranks are 1..N contiguous per (period, rank_type).
func TestInvariantI7_RankSequenceContiguous(t *testing.T) {
	t.Skip("pending: requires P5 ranking/compute, P6 ranking/invariants")
}

// I8: rankings.json has exactly the 6 keys (1d/7d/30d × breakout/trending).
func TestInvariantI8_RankingsJSONHasSixKeys(t *testing.T) {
	t.Skip("pending: requires P14 export/json")
}

// I9: JSON Marshal -> Unmarshal yields equal structs (RepoDetail/Rankings/Meta).
func TestInvariantI9_JSONRoundTrip(t *testing.T) {
	t.Skip("pending: requires P13 export/schema")
}

// I10: Migrate is idempotent. (Already enforced in db/schema_test.go.)
func TestInvariantI10_MigrateIdempotent(t *testing.T) {
	t.Skip("pending: covered by db.TestMigrateIdempotent — to be promoted into pipeline integration test")
}

// I11: data-only restore reproduces the same JSON output bit-for-bit.
func TestInvariantI11_DataIsSourceOfTruth(t *testing.T) {
	t.Skip("pending: requires P12 restore, P14 export/json, P16 pipeline/compute")
}

// I12: empty rankings across all 6 slots aborts the pipeline (exit 1).
func TestInvariantI12_EmptyRankingsAborts(t *testing.T) {
	t.Skip("pending: requires P16 pipeline/compute")
}

// I13: re-export with same DB state and same computed_date is byte-identical
// (modulo updated_at / generated_at fields).
func TestInvariantI13_DeterministicExport(t *testing.T) {
	t.Skip("pending: requires P14 export/json")
}
