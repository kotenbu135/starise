// Package discover expands the repository universe beyond seeds.txt by
// issuing GitHub Search queries across star ranges, languages, and topics.
//
// Design notes (see CLAUDE.md / rewrite plan):
//   - Search API only (GraphQL). Trending scraping was removed as unreliable.
//   - All queries hard-code fork:false archived:false so results are already
//     filtered at source.
//   - Run is deterministic given a fixed BuildQueries() — enables replayable
//     tests against a mocked client.
package discover

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// Stats summarizes a discover run.
type Stats struct {
	Added    int // unique repos persisted this run
	Skipped  int // archived / forked
	Failed   int // API errors
	Queries  int // search queries executed
	Requests int // pages fetched across all queries
}

// Base filter applied to every query.
const baseFilter = "fork:false archived:false"

// starRanges segment the >=100 stars space to avoid the 1000-result cap.
var starRanges = []string{
	"stars:>=50000",
	"stars:20000..49999",
	"stars:10000..19999",
	"stars:5000..9999",
	"stars:2000..4999",
	"stars:1000..1999",
	"stars:500..999",
	"stars:200..499",
	"stars:100..199",
}

// languages is the subset scanned per-language (each further segmented by stars).
var languages = []string{
	"Python", "TypeScript", "JavaScript", "Go", "Rust",
	"Java", "C++", "C#", "Swift", "Kotlin",
	"Dart", "Ruby", "PHP", "Scala", "Elixir",
}

// langStarRanges is the per-language star segmentation (coarser than global).
var langStarRanges = []string{
	"stars:>=10000",
	"stars:1000..9999",
	"stars:100..999",
}

// BuildQueries returns the static list of GitHub Search queries executed
// by Run. Pure function: no env, no clock.
func BuildQueries() []string {
	var out []string
	for _, sr := range starRanges {
		out = append(out, fmt.Sprintf("%s %s", baseFilter, sr))
	}
	for _, lang := range languages {
		for _, sr := range langStarRanges {
			out = append(out, fmt.Sprintf("%s language:%s %s", baseFilter, lang, sr))
		}
	}
	return out
}

// Run executes the given queries against client, paginating up to maxPages,
// and persists discovered repos + today's star snapshot. Returns aggregate
// stats. Partial failures are logged; the run completes best-effort.
func Run(client github.Client, database *sql.DB, queries []string, today string, maxPages int) (Stats, error) {
	stats := Stats{Queries: len(queries)}
	seen := make(map[string]bool)

	for _, q := range queries {
		var cursor string
		for page := 0; page < maxPages; page++ {
			res, err := client.SearchRepos(q, 100, cursor)
			stats.Requests++
			if err != nil {
				log.Printf("discover: %q page %d: %v", q, page, err)
				stats.Failed++
				break
			}

			for _, repo := range res.Repos {
				if repo.IsArchived || repo.IsFork {
					stats.Skipped++
					continue
				}
				key := repo.Owner.Login + "/" + repo.Name
				if seen[key] {
					continue
				}
				if err := saveDiscovered(database, repo, today); err != nil {
					log.Printf("discover: save %s: %v", key, err)
					stats.Failed++
					continue
				}
				seen[key] = true
				stats.Added++
			}

			if !res.HasNext {
				break
			}
			cursor = res.EndCursor
		}
	}
	return stats, nil
}

func saveDiscovered(database *sql.DB, r github.RepoData, today string) error {
	row := toDBRepository(r)
	id, err := db.UpsertRepository(database, row)
	if err != nil {
		return err
	}
	return db.UpsertDailyStar(database, &db.DailyStar{
		RepoID:       id,
		RecordedDate: today,
		StarCount:    r.StargazerCount,
	})
}

func toDBRepository(r github.RepoData) *db.Repository {
	row := &db.Repository{
		GitHubID:   r.ID,
		Owner:      r.Owner.Login,
		Name:       r.Name,
		URL:        r.URL,
		ForkCount:  r.ForkCount,
		IsArchived: r.IsArchived,
		IsFork:     r.IsFork,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		PushedAt:   r.PushedAt,
	}
	if r.Description != nil {
		row.Description = *r.Description
	}
	if r.HomepageURL != nil {
		row.HomepageURL = *r.HomepageURL
	}
	if r.PrimaryLang != nil {
		row.Language = r.PrimaryLang.Name
	}
	if r.LicenseInfo != nil {
		row.License = r.LicenseInfo.Name
	}
	topics := make([]string, 0, len(r.Topics))
	for _, t := range r.Topics {
		topics = append(topics, t.Name)
	}
	if b, err := json.Marshal(topics); err == nil {
		row.Topics = string(b)
	} else {
		row.Topics = "[]"
	}
	return row
}
