// Package fetch pulls repository metadata + the current star count from
// GitHub and persists daily snapshots. The real work is a single pure-ish
// function, Seeds, that accepts any github.Client so tests use a mock.
package fetch

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// Stats summarizes the outcome of a fetch run.
type Stats struct {
	Fetched int // repos written to DB
	Skipped int // archived / forked / intentionally ignored
	Failed  int // API errors, malformed seeds
}

// ParseSeedsText parses a seeds.txt body: one "owner/name" per line, blanks
// and lines starting with '#' ignored. Whitespace trimmed.
func ParseSeedsText(body string) ([]string, error) {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// Seeds fetches each "owner/name" seed, persisting the repository record and
// today's daily_stars snapshot. Failures are logged and counted but never
// abort the whole run.
//
// today must be in YYYY-MM-DD form (UTC anchor, controlled by the caller).
func Seeds(client github.Client, database *sql.DB, seeds []string, today string) (Stats, error) {
	var stats Stats
	for _, seed := range seeds {
		owner, name, ok := splitSeed(seed)
		if !ok {
			log.Printf("fetch: malformed seed %q", seed)
			stats.Failed++
			continue
		}

		res, err := client.FetchRepo(owner, name)
		if err != nil {
			if errors.Is(err, github.ErrNotFound) {
				log.Printf("fetch: %s/%s not found", owner, name)
			} else {
				log.Printf("fetch: %s/%s: %v", owner, name, err)
			}
			stats.Failed++
			continue
		}

		if res.Repo.IsArchived || res.Repo.IsFork {
			stats.Skipped++
			continue
		}

		if err := saveRepoAndStar(database, res.Repo, today); err != nil {
			log.Printf("fetch: save %s/%s: %v", owner, name, err)
			stats.Failed++
			continue
		}
		stats.Fetched++
	}
	return stats, nil
}

// saveRepoAndStar upserts the repository record and today's star snapshot.
func saveRepoAndStar(database *sql.DB, r github.RepoData, today string) error {
	row := toDBRepository(r)
	id, err := db.UpsertRepository(database, row)
	if err != nil {
		return fmt.Errorf("upsert repo: %w", err)
	}
	return db.UpsertDailyStar(database, &db.DailyStar{
		RepoID:       id,
		RecordedDate: today,
		StarCount:    r.StargazerCount,
	})
}

// toDBRepository converts a github.RepoData into the DB row shape.
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

func splitSeed(seed string) (owner, name string, ok bool) {
	parts := strings.SplitN(seed, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
