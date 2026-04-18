package fetch

import (
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	_ "modernc.org/sqlite"
)

func TestFetchSeedsUpsertsReposAndStars(t *testing.T) {
	database, err := db.Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mock := github.NewMock()
	mock.StubRepo("acme", "widget", github.RepoData{
		ID: "g1", Owner: github.Owner{Login: "acme"}, Name: "widget",
		URL: "https://github.com/acme/widget", StargazerCount: 100,
	})
	mock.StubRepo("foo", "bar", github.RepoData{
		ID: "g2", Owner: github.Owner{Login: "foo"}, Name: "bar",
		URL: "https://github.com/foo/bar", StargazerCount: 50,
	})

	stats, err := Seeds(mock, database, []string{"acme/widget", "foo/bar"}, "2026-04-18")
	if err != nil {
		t.Fatalf("seeds: %v", err)
	}
	if stats.Fetched != 2 || stats.Failed != 0 {
		t.Errorf("stats unexpected: %+v", stats)
	}

	// Verify DB state.
	r, err := db.GetRepositoryByOwnerName(database, "acme", "widget")
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	s, err := db.GetDailyStar(database, r.ID, "2026-04-18")
	if err != nil {
		t.Fatalf("get star: %v", err)
	}
	if s.StarCount != 100 {
		t.Errorf("stars: %d want 100", s.StarCount)
	}
}

func TestFetchSeedsSkipsArchivedAndForks(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	mock.StubRepo("x", "archived", github.RepoData{
		ID: "g1", Owner: github.Owner{Login: "x"}, Name: "archived",
		IsArchived: true, StargazerCount: 10,
	})
	mock.StubRepo("x", "fork", github.RepoData{
		ID: "g2", Owner: github.Owner{Login: "x"}, Name: "fork",
		IsFork: true, StargazerCount: 10,
	})
	mock.StubRepo("x", "ok", github.RepoData{
		ID: "g3", Owner: github.Owner{Login: "x"}, Name: "ok",
		StargazerCount: 20,
	})

	stats, err := Seeds(mock, database, []string{"x/archived", "x/fork", "x/ok"}, "2026-04-18")
	if err != nil {
		t.Fatalf("seeds: %v", err)
	}
	if stats.Fetched != 1 || stats.Skipped != 2 {
		t.Errorf("stats unexpected: %+v", stats)
	}
}

func TestFetchSeedsRecordsFailuresAndContinues(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	// Only stub one — the other will ErrNotFound.
	mock.StubRepo("a", "b", github.RepoData{
		ID: "g1", Owner: github.Owner{Login: "a"}, Name: "b", StargazerCount: 5,
	})

	stats, err := Seeds(mock, database, []string{"a/b", "missing/missing"}, "2026-04-18")
	if err != nil {
		t.Fatalf("seeds returned error on partial failure: %v", err)
	}
	if stats.Fetched != 1 || stats.Failed != 1 {
		t.Errorf("stats unexpected: %+v", stats)
	}
}

func TestFetchSeedsRejectsMalformedSeed(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	stats, err := Seeds(mock, database, []string{"not-a-slash-separated"}, "2026-04-18")
	if err != nil {
		t.Fatalf("seeds: %v", err)
	}
	if stats.Fetched != 0 || stats.Failed != 1 {
		t.Errorf("expected 1 failure for malformed, got %+v", stats)
	}
}

func TestParseSeedsTxtIgnoresBlanksAndComments(t *testing.T) {
	input := `
# comment line
acme/widget

   foo/bar
# another
	baz/qux
`
	got, err := ParseSeedsText(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"acme/widget", "foo/bar", "baz/qux"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}
