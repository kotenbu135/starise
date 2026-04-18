package refresh

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

func TestRefreshUpdatesAllAndZeroFailures(t *testing.T) {
	d, err := db.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	c := github.NewMockClient()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("G%d", i)
		db.UpsertRepository(d, db.Repository{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i)})
		c.Add(github.RepoData{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i), StarCount: 100 + i})
	}

	res, err := Run(context.Background(), d, c, "2026-04-18", DefaultMaxFailureRate)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Refreshed != 5 || res.SoftDeleted != 0 {
		t.Errorf("res=%+v", res)
	}
	if res.FailureRate != 0 {
		t.Errorf("rate=%v", res.FailureRate)
	}
}

func TestRefreshSoftDeletesMissing(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	// 10 total, 2 missing → 20% failure (under 30%)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("G%d", i)
		db.UpsertRepository(d, db.Repository{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i)})
		if i < 2 {
			c.MissingIDs[id] = true
			continue
		}
		c.Add(github.RepoData{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i), StarCount: 100})
	}

	res, err := Run(context.Background(), d, c, "2026-04-18", DefaultMaxFailureRate)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.SoftDeleted != 2 || res.Refreshed != 8 {
		t.Errorf("res=%+v", res)
	}
	r0, _ := db.GetRepositoryByGitHubID(d, "G0")
	if r0.DeletedAt != "2026-04-18" {
		t.Errorf("G0 not soft-deleted: %q", r0.DeletedAt)
	}
}

func TestRefreshAbortsAboveThreshold(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	// 10 total, 4 missing → 40% (above 30%)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("G%d", i)
		db.UpsertRepository(d, db.Repository{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i)})
		if i < 4 {
			c.MissingIDs[id] = true
			continue
		}
		c.Add(github.RepoData{GitHubID: id, Owner: "x", Name: fmt.Sprintf("r%d", i), StarCount: 100})
	}

	res, err := Run(context.Background(), d, c, "2026-04-18", DefaultMaxFailureRate)
	if !errors.Is(err, ErrFailureRateExceeded) {
		t.Errorf("got %v, want ErrFailureRateExceeded", err)
	}
	if res.FailureRate <= DefaultMaxFailureRate {
		t.Errorf("rate=%v, expected above threshold", res.FailureRate)
	}
}

func TestRefreshDetectsArchivedFlip(t *testing.T) {
	d, _ := db.Open("")
	defer d.Close()
	c := github.NewMockClient()
	db.UpsertRepository(d, db.Repository{GitHubID: "G1", Owner: "x", Name: "a", IsArchived: false})
	c.Add(github.RepoData{GitHubID: "G1", Owner: "x", Name: "a", IsArchived: true, StarCount: 100})

	res, err := Run(context.Background(), d, c, "2026-04-18", DefaultMaxFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	if res.ArchivedFlipped != 1 {
		t.Errorf("flips=%d", res.ArchivedFlipped)
	}
	r, _ := db.GetRepositoryByGitHubID(d, "G1")
	if !r.IsArchived {
		t.Errorf("archived not persisted")
	}
}
