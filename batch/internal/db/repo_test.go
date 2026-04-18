package db

import (
	"testing"
)

func TestUpsertRepositoryInsertsNew(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	r := &Repository{
		GitHubID: "MDEwOlJlcG9zaXRvcnkx",
		Owner:    "acme",
		Name:     "widget",
		Language: "Go",
	}

	id, err := UpsertRepository(d, r)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id should be positive, got %d", id)
	}

	got, err := GetRepositoryByOwnerName(d, "acme", "widget")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID mismatch: %d vs %d", got.ID, id)
	}
	if got.Language != "Go" {
		t.Errorf("language: got %q", got.Language)
	}
}

func TestUpsertRepositoryUpdatesExisting(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	first := &Repository{GitHubID: "gid1", Owner: "o", Name: "r", Description: "old"}
	id1, err := UpsertRepository(d, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second := &Repository{GitHubID: "gid1", Owner: "o", Name: "r", Description: "new"}
	id2, err := UpsertRepository(d, second)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if id1 != id2 {
		t.Errorf("id should be stable on upsert: %d vs %d", id1, id2)
	}

	got, _ := GetRepositoryByOwnerName(d, "o", "r")
	if got.Description != "new" {
		t.Errorf("description not updated: %q", got.Description)
	}
}

func TestGetRepositoryMissingReturnsNotFound(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	_, err := GetRepositoryByOwnerName(d, "nope", "nope")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListRepositoriesReturnsAll(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	for i, name := range []string{"a", "b", "c"} {
		_, err := UpsertRepository(d, &Repository{
			GitHubID: string(rune('a' + i)),
			Owner:    "o",
			Name:     name,
		})
		if err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}

	list, err := ListRepositories(d)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3, got %d", len(list))
	}
}
