package pipeline

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
)

// I10: Migrate is idempotent at the pipeline boundary.
// Covered atomically in db/schema_test.go; pinned here so a future change
// to schema.sql that breaks idempotency fails this integration test too.
func TestInvariantI10_MigrateIdempotent_Real(t *testing.T) {
	d := openMem(t)
	for i := 0; i < 5; i++ {
		if err := db.Migrate(d); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
}
