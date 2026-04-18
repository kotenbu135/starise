package db

import (
	"testing"
)

func TestUpsertDailyStarsInsertAndOverwrite(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	id, _ := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "b"})

	if err := UpsertDailyStar(d, id, "2026-04-18", 100); err != nil {
		t.Fatal(err)
	}
	if err := UpsertDailyStar(d, id, "2026-04-18", 150); err != nil {
		t.Fatal(err)
	}

	stars, err := ListStarHistory(d, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(stars) != 1 {
		t.Fatalf("len=%d", len(stars))
	}
	if stars[0].StarCount != 150 {
		t.Errorf("overwrite failed: %d", stars[0].StarCount)
	}
}

func TestStarsAtOrBefore(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	id, _ := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "b"})
	UpsertDailyStar(d, id, "2026-04-10", 100)
	UpsertDailyStar(d, id, "2026-04-15", 200)
	UpsertDailyStar(d, id, "2026-04-18", 300)

	cases := map[string]struct {
		date string
		want int
		ok   bool
	}{
		"exact":  {"2026-04-15", 200, true},
		"before": {"2026-04-12", 100, true},
		"after":  {"2026-04-20", 300, true},
		"none":   {"2026-04-09", 0, false},
	}
	for name, tc := range cases {
		got, ok, err := StarCountAtOrBefore(d, id, tc.date)
		if err != nil {
			t.Errorf("%s: err %v", name, err)
		}
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: got (%d,%v), want (%d,%v)", name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestListStarHistoryOrdered(t *testing.T) {
	d := openMem(t)
	Migrate(d)
	id, _ := UpsertRepository(d, Repository{GitHubID: "G1", Owner: "a", Name: "b"})
	UpsertDailyStar(d, id, "2026-04-15", 200)
	UpsertDailyStar(d, id, "2026-04-10", 100)
	UpsertDailyStar(d, id, "2026-04-18", 300)

	hist, _ := ListStarHistory(d, id)
	want := []string{"2026-04-10", "2026-04-15", "2026-04-18"}
	for i, h := range hist {
		if h.RecordedDate != want[i] {
			t.Errorf("idx %d: %s vs %s", i, h.RecordedDate, want[i])
		}
	}
}
