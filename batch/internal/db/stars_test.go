package db

import (
	"testing"
)

func TestUpsertDailyStarInsertsNew(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoID, err := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}

	err = UpsertDailyStar(d, &DailyStar{
		RepoID:       repoID,
		RecordedDate: "2026-04-18",
		StarCount:    1234,
	})
	if err != nil {
		t.Fatalf("star: %v", err)
	}

	got, err := GetDailyStar(d, repoID, "2026-04-18")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.StarCount != 1234 {
		t.Errorf("star count: %d", got.StarCount)
	}
}

func TestUpsertDailyStarOverwrites(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoID, _ := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})

	_ = UpsertDailyStar(d, &DailyStar{RepoID: repoID, RecordedDate: "2026-04-18", StarCount: 100})
	err := UpsertDailyStar(d, &DailyStar{RepoID: repoID, RecordedDate: "2026-04-18", StarCount: 200})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, _ := GetDailyStar(d, repoID, "2026-04-18")
	if got.StarCount != 200 {
		t.Errorf("expected 200, got %d", got.StarCount)
	}
}

func TestGetDailyStarMissingReturnsNotFound(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoID, _ := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})
	_, err := GetDailyStar(d, repoID, "2099-12-31")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListDailyStarsOrderedByDateAsc(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoID, _ := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})

	for _, s := range []DailyStar{
		{RepoID: repoID, RecordedDate: "2026-04-17", StarCount: 10},
		{RepoID: repoID, RecordedDate: "2026-04-15", StarCount: 5},
		{RepoID: repoID, RecordedDate: "2026-04-18", StarCount: 20},
	} {
		if err := UpsertDailyStar(d, &s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	list, err := ListDailyStars(d, repoID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].RecordedDate != "2026-04-15" || list[2].RecordedDate != "2026-04-18" {
		t.Errorf("not ordered asc: %+v", list)
	}
}

func TestGetStarAtOrBeforeReturnsClosestPrior(t *testing.T) {
	d := openMemory(t)
	mustMigrate(t, d)

	repoID, _ := UpsertRepository(d, &Repository{GitHubID: "g", Owner: "o", Name: "r"})

	for _, s := range []DailyStar{
		{RepoID: repoID, RecordedDate: "2026-04-10", StarCount: 50},
		{RepoID: repoID, RecordedDate: "2026-04-15", StarCount: 100},
		{RepoID: repoID, RecordedDate: "2026-04-18", StarCount: 200},
	} {
		_ = UpsertDailyStar(d, &s)
	}

	// exact match
	got, err := GetStarAtOrBefore(d, repoID, "2026-04-15")
	if err != nil {
		t.Fatalf("exact: %v", err)
	}
	if got.StarCount != 100 {
		t.Errorf("exact: got %d want 100", got.StarCount)
	}

	// between -> returns 2026-04-15 (latest before target)
	got, err = GetStarAtOrBefore(d, repoID, "2026-04-17")
	if err != nil {
		t.Fatalf("between: %v", err)
	}
	if got.RecordedDate != "2026-04-15" {
		t.Errorf("between: got date %s want 2026-04-15", got.RecordedDate)
	}

	// before all -> not found
	_, err = GetStarAtOrBefore(d, repoID, "2026-01-01")
	if err != ErrNotFound {
		t.Errorf("before all: expected ErrNotFound, got %v", err)
	}
}
