package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/refresh"
)

// I4: refresh tolerates partial failures (≤30%) but exits non-zero above the
// threshold. Verified directly through the refresh package as well; this
// integration test pins the contract at the pipeline boundary.
func TestInvariantI4_RefreshFailureTolerance_Real(t *testing.T) {
	cases := []struct {
		name        string
		missing     int
		total       int
		expectAbort bool
	}{
		{"20% missing — tolerated", 20, 100, false},
		{"40% missing — abort", 40, 100, true},
		{"30% missing — exact threshold tolerated", 30, 100, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := openMem(t)
			c := github.NewMockClient()
			for i := 0; i < tc.total; i++ {
				id := fmt.Sprintf("G%d", i)
				db.UpsertRepository(d, db.Repository{GitHubID: id, Owner: "x", Name: id})
				if i < tc.missing {
					c.MissingIDs[id] = true
					continue
				}
				c.Add(github.RepoData{GitHubID: id, Owner: "x", Name: id, StarCount: 100})
			}
			_, err := refresh.Run(context.Background(), d, c, "2026-04-18", refresh.DefaultMaxFailureRate)
			gotAbort := errors.Is(err, refresh.ErrFailureRateExceeded)
			if gotAbort != tc.expectAbort {
				t.Errorf("got abort=%v, want %v (err=%v)", gotAbort, tc.expectAbort, err)
			}
		})
	}
}
