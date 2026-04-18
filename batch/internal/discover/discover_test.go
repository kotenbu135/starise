package discover

import (
	"strings"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	_ "modernc.org/sqlite"
)

func TestBuildQueriesContainsStarRangesAndLanguages(t *testing.T) {
	queries := BuildQueries()
	if len(queries) == 0 {
		t.Fatal("no queries generated")
	}
	// Sanity checks: at least one star-range-only query, one per-language query.
	haveStarOnly := false
	haveLangGo := false
	for _, q := range queries {
		if strings.Contains(q, "stars:>=") && !strings.Contains(q, "language:") {
			haveStarOnly = true
		}
		if strings.Contains(q, "language:Go") {
			haveLangGo = true
		}
	}
	if !haveStarOnly {
		t.Errorf("missing star-only query")
	}
	if !haveLangGo {
		t.Errorf("missing language:Go query")
	}
}

func TestBuildQueriesAlwaysExcludesForksAndArchived(t *testing.T) {
	for _, q := range BuildQueries() {
		if !strings.Contains(q, "fork:false") {
			t.Errorf("query missing fork:false: %q", q)
		}
		if !strings.Contains(q, "archived:false") {
			t.Errorf("query missing archived:false: %q", q)
		}
	}
}

func TestRunPersistsReposAndStars(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	// Stub every generated query with 1 repo so we can measure dedup + saves.
	queries := BuildQueries()
	for i, q := range queries {
		mock.StubSearch(q, []github.RepoData{
			{
				ID:             "gid" + itoa(i),
				Owner:          github.Owner{Login: "o" + itoa(i)},
				Name:           "r" + itoa(i),
				StargazerCount: 100 + i,
			},
		})
	}

	stats, err := Run(mock, database, queries, "2026-04-18", 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.Added != len(queries) {
		t.Errorf("added=%d want %d", stats.Added, len(queries))
	}
	list, _ := db.ListRepositories(database)
	if len(list) != len(queries) {
		t.Errorf("repositories persisted=%d want %d", len(list), len(queries))
	}
}

func TestRunSkipsArchivedAndForks(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	q := "fork:false archived:false stars:>=1000"
	mock.StubSearch(q, []github.RepoData{
		{ID: "g1", Owner: github.Owner{Login: "o"}, Name: "archived", IsArchived: true},
		{ID: "g2", Owner: github.Owner{Login: "o"}, Name: "fork", IsFork: true},
		{ID: "g3", Owner: github.Owner{Login: "o"}, Name: "ok", StargazerCount: 500},
	})

	stats, err := Run(mock, database, []string{q}, "2026-04-18", 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.Added != 1 || stats.Skipped != 2 {
		t.Errorf("stats %+v", stats)
	}
}

func TestRunDedupsAcrossQueries(t *testing.T) {
	database, _ := db.Open("")
	defer database.Close()
	_ = db.Migrate(database)

	mock := github.NewMock()
	repo := github.RepoData{
		ID: "g", Owner: github.Owner{Login: "o"}, Name: "r", StargazerCount: 42,
	}
	mock.StubSearch("q1", []github.RepoData{repo})
	mock.StubSearch("q2", []github.RepoData{repo})

	stats, err := Run(mock, database, []string{"q1", "q2"}, "2026-04-18", 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Same repo seen twice; upsert is idempotent — only one DB row.
	list, _ := db.ListRepositories(database)
	if len(list) != 1 {
		t.Errorf("expected 1 repo after dedup, got %d", len(list))
	}
	if stats.Added != 1 {
		// Added should count unique repos, not search hits.
		t.Errorf("added=%d want 1 (dedup)", stats.Added)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
