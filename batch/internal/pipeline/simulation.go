package pipeline

// SimulationStep advances the data dir by one day. It is the helper used by
// invariant_i3_test.go so the multi-day round-trip can be expressed without
// duplicating the boilerplate inline.
//
// The flow per day is:
//   1. Open a fresh in-memory DB.
//   2. Restore from dataDir (so prior days' history is loaded).
//   3. Run the pipeline with the given Today + mock Client.
//   4. Close the DB. The resulting JSON in dataDir is the state for next day.
//
// Note: This intentionally re-opens the DB each day to mirror how the
// production CI run starts fresh from the data/ tree (issue #2 I11).

import (
	"context"

	"github.com/kotenbu135/starise/batch/internal/db"
)

// RunSimulationDay performs one synthetic pipeline day against dataDir.
// Returns the report so tests can assert per-day counts.
func RunSimulationDay(ctx context.Context, dataDir string, opts Options) (RunReport, error) {
	d, err := db.Open("")
	if err != nil {
		return RunReport{}, err
	}
	defer d.Close()

	opts.RestoreFrom = dataDir
	opts.OutDir = dataDir
	return RunAll(ctx, d, opts)
}
