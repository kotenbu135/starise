// Package restore rebuilds the SQLite DB from the JSON tree under data/repos/.
//
// data/ is the source of truth (issue #2 I11). Restore + compute + export
// from a fresh DB must reproduce the same JSON output as the original run.
package restore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
)

type Result struct {
	Repos    int
	Snapshots int
}

// FromDir scans dir/repos/*.json and upserts each into the DB. Star history
// is replayed in date order. deleted_at is preserved.
func FromDir(d *sql.DB, dir string) (Result, error) {
	repoDir := filepath.Join(dir, "repos")
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", repoDir, err)
	}

	res := Result{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(repoDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return res, fmt.Errorf("read %s: %w", path, err)
		}
		var rd export.RepoDetail
		if err := json.Unmarshal(b, &rd); err != nil {
			return res, fmt.Errorf("unmarshal %s: %w", path, err)
		}
		repo := db.Repository{
			GitHubID:    rd.RepoID,
			Owner:       strings.ToLower(rd.Owner),
			Name:        strings.ToLower(rd.Name),
			Description: rd.Description,
			URL:         rd.URL,
			HomepageURL: rd.HomepageURL,
			Language:    rd.Language,
			License:     rd.License,
			Topics:      rd.Topics,
			IsArchived:  rd.IsArchived,
			IsFork:      rd.IsFork,
			ForkCount:   rd.ForkCount,
			CreatedAt:   rd.CreatedAt,
			UpdatedAt:   rd.UpdatedAt,
			PushedAt:    rd.PushedAt,
		}
		id, err := db.UpsertRepository(d, repo)
		if err != nil {
			return res, fmt.Errorf("upsert %s/%s: %w", repo.Owner, repo.Name, err)
		}
		// Preserve deleted_at separately (UpsertRepository doesn't write it).
		if rd.DeletedAt != "" {
			if err := db.SoftDeleteByGitHubID(d, repo.GitHubID, rd.DeletedAt); err != nil {
				return res, fmt.Errorf("soft delete %s: %w", repo.GitHubID, err)
			}
		}
		for _, h := range rd.StarHistory {
			if err := db.UpsertDailyStar(d, id, h.Date, h.Stars); err != nil {
				return res, fmt.Errorf("upsert star %s @ %s: %w", repo.GitHubID, h.Date, err)
			}
			res.Snapshots++
		}
		res.Repos++
	}
	return res, nil
}
