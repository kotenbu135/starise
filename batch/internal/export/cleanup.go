package export

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
)

// HardDeleteAfterDays controls how long soft-deleted repos remain in the DB
// before they are permanently removed (DB row + JSON file). Per issue #2.
const HardDeleteAfterDays = 90

type CleanupResult struct {
	OrphansRemoved   int // JSON files with no matching DB row
	HardDeleted      int // soft-deleted >= HardDeleteAfterDays ago
}

// Cleanup performs:
//  1. Removes data/repos/*.json files whose owner/name combination no longer
//     exists in the DB (e.g. after a hard delete).
//  2. For repos with deleted_at set and >= HardDeleteAfterDays old, removes
//     them from the DB and deletes their JSON file.
//
// today must be YYYY-MM-DD; this is the reference for the 90-day window.
func Cleanup(d *sql.DB, outDir, today string) (CleanupResult, error) {
	res := CleanupResult{}
	repoDir := filepath.Join(outDir, "repos")

	// Step 2 first — hard delete passed-window soft-deletes BEFORE the orphan
	// scan so the JSON files they leave behind get removed in the same call.
	cutoff, err := time.Parse("2006-01-02", today)
	if err != nil {
		return res, fmt.Errorf("today: %w", err)
	}
	cutoff = cutoff.AddDate(0, 0, -HardDeleteAfterDays)
	cutoffStr := cutoff.Format("2006-01-02")

	all, err := db.ListAllRepositories(d)
	if err != nil {
		return res, err
	}
	for _, r := range all {
		if r.DeletedAt == "" || r.DeletedAt > cutoffStr {
			continue
		}
		if err := db.HardDeleteByGitHubID(d, r.GitHubID); err != nil {
			continue
		}
		// Remove its JSON if present.
		path := filepath.Join(repoDir, r.Owner+"__"+r.Name+".json")
		_ = os.Remove(path)
		res.HardDeleted++
	}

	// Step 1 — sweep orphan JSON files.
	keep := map[string]bool{}
	all, err = db.ListAllRepositories(d)
	if err != nil {
		return res, err
	}
	for _, r := range all {
		keep[r.Owner+"__"+r.Name+".json"] = true
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if keep[name] {
			continue
		}
		_ = os.Remove(filepath.Join(repoDir, name))
		res.OrphansRemoved++
	}
	return res, nil
}
