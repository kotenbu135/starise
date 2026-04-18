package db

import (
	"reflect"
	"sort"
	"testing"
)

func TestUpsertRepositoryInsertAndUpdate(t *testing.T) {
	d := openMem(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	r := Repository{
		GitHubID: "G1", Owner: "acme", Name: "widget",
		Description: "first", URL: "https://github.com/acme/widget",
		Language: "Go", License: "MIT", Topics: []string{"ai"},
		ForkCount: 5, IsArchived: false, IsFork: false,
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2026-04-18T00:00:00Z",
		PushedAt:  "2026-04-18T00:00:00Z",
	}
	id1, err := UpsertRepository(d, r)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("id1=%d", id1)
	}

	r.Description = "second"
	r.ForkCount = 10
	id2, err := UpsertRepository(d, r)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if id1 != id2 {
		t.Errorf("id changed on upsert: %d -> %d", id1, id2)
	}

	got, err := GetRepositoryByGitHubID(d, "G1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "second" || got.ForkCount != 10 {
		t.Errorf("update not applied: %+v", got)
	}
	if !reflect.DeepEqual(got.Topics, []string{"ai"}) {
		t.Errorf("topics: %v", got.Topics)
	}
}

func TestSoftDeleteRepository(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "b"})
	if err := SoftDeleteByGitHubID(d, "G1", "2026-04-18"); err != nil {
		t.Fatal(err)
	}
	got, _ := GetRepositoryByGitHubID(d, "G1")
	if got.DeletedAt != "2026-04-18" {
		t.Errorf("deleted_at = %q", got.DeletedAt)
	}
}

// ListActiveRepositories returns only non-archived, non-deleted repos.
// Per issue I1: rankings.json shows "active only" (= non-archived, non-deleted).
func TestListActiveRepositories(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "live"})
	UpsertRepository(d, Repository{GitHubID: "G2", Owner: "a", Name: "archived", IsArchived: true})
	UpsertRepository(d, Repository{GitHubID: "G3", Owner: "a", Name: "soft_deleted"})
	SoftDeleteByGitHubID(d, "G3", "2026-04-18")

	repos, err := ListActiveRepositories(d)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, r := range repos {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	want := []string{"live"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestListAllNonDeletedRepositories(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "x"})
	UpsertRepository(d, Repository{GitHubID: "G2", Owner: "a", Name: "y", IsArchived: true})
	UpsertRepository(d, Repository{GitHubID: "G3", Owner: "a", Name: "z"})
	SoftDeleteByGitHubID(d, "G3", "2026-04-18")

	all, err := ListNonDeletedRepositories(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("non-deleted count = %d", len(all))
	}
}

func TestOwnerNameLowercaseStored(t *testing.T) {
	// Caller-side normalization is contract; the DB stores what is given.
	// We assert UNIQUE constraint sees mixed case as different — caller MUST normalize.
	d := openMem(t)
	Migrate(d)
	if _, err := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "acme", Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertRepository(d, Repository{GitHubID: "G2", Owner: "ACME", Name: "X"}); err != nil {
		t.Fatalf("expected separate row for ACME/X: %v", err)
	}
}
