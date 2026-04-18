// Package restore rehydrates the SQLite DB from data/repos/*.json.
//
// This is the "source of truth" path: data/ is committed to git, so even
// when GitHub Actions starts with a fresh runner, yesterday's daily_stars
// history is recovered from the repo detail JSON files before the day's
// fetch/compute runs.
package restore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
)

// Stats summarizes a restore run.
type Stats struct {
	Files      int // JSON files scanned (including malformed)
	Repos      int // repositories upserted
	StarPoints int // daily_stars rows upserted
	Failed     int // files that failed to parse or persist
}

// FromDir restores DB state from dir/repos/*.json. dir is the data/ root
// (i.e. it should contain a repos/ subdir). Missing repos/ is not an error —
// the function returns zero stats and nil (fresh-install case).
//
// All writes go through the normal Upsert path, so re-running is idempotent.
// Parse failures are logged and counted but never abort the run: one bad
// file must not block the rest of the restore.
func FromDir(d *sql.DB, dir string) (Stats, error) {
	var stats Stats

	if info, err := os.Stat(dir); err != nil {
		return stats, fmt.Errorf("stat %s: %w", dir, err)
	} else if !info.IsDir() {
		return stats, fmt.Errorf("%s is not a directory", dir)
	}

	reposDir := filepath.Join(dir, "repos")
	info, err := os.Stat(reposDir)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil // nothing to restore
		}
		return stats, fmt.Errorf("stat %s: %w", reposDir, err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("%s is not a directory", reposDir)
	}

	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return stats, fmt.Errorf("read dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		stats.Files++
		path := filepath.Join(reposDir, e.Name())
		added, starPoints, err := restoreFile(d, path)
		if err != nil {
			log.Printf("restore: %s: %v", e.Name(), err)
			stats.Failed++
			continue
		}
		stats.Repos += added
		stats.StarPoints += starPoints
	}
	return stats, nil
}

// restoreFile parses one JSON file and upserts its repository + star history.
// Returns (reposAdded, starPointsAdded, err). reposAdded is 0 or 1.
func restoreFile(d *sql.DB, path string) (int, int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("read: %w", err)
	}

	var detail export.RepoDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return 0, 0, fmt.Errorf("parse: %w", err)
	}
	if detail.Owner == "" || detail.Name == "" {
		return 0, 0, fmt.Errorf("missing owner/name")
	}

	topicsJSON := "[]"
	if len(detail.Topics) > 0 {
		if b, err := json.Marshal(detail.Topics); err == nil {
			topicsJSON = string(b)
		}
	}

	id, err := db.UpsertRepository(d, &db.Repository{
		GitHubID:    detail.RepoID,
		Owner:       detail.Owner,
		Name:        detail.Name,
		Description: detail.Description,
		URL:         detail.URL,
		HomepageURL: detail.HomepageURL,
		Language:    detail.Language,
		License:     detail.License,
		Topics:      topicsJSON,
		ForkCount:   detail.ForkCount,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("upsert repo: %w", err)
	}

	points := 0
	for _, p := range detail.StarHistory {
		if p.Date == "" {
			continue
		}
		if err := db.UpsertDailyStar(d, &db.DailyStar{
			RepoID:       id,
			RecordedDate: p.Date,
			StarCount:    p.Stars,
		}); err != nil {
			return 1, points, fmt.Errorf("upsert star %s: %w", p.Date, err)
		}
		points++
	}
	return 1, points, nil
}
