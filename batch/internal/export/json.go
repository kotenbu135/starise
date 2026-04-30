package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// Options control the export run.
type Options struct {
	OutDir       string
	UpdatedAt    string // RFC3339, written verbatim into rankings.json
	GeneratedAt  string // RFC3339, written verbatim into meta.json
	ComputedDate string // YYYY-MM-DD; selects which rankings rows to include
	TopN         int    // safety cap; rankings table is already capped
	// TranslationCacheDir, when non-empty, points at the on-disk
	// translate.Cache root (e.g. data/translations). Each repo's
	// description is hashed and looked up; hits populate DescriptionJA,
	// misses leave it empty and the frontend renders the original
	// English description as fallback.
	TranslationCacheDir string
}

// Export writes data/repos/*.json + data/rankings.json + data/meta.json.
// Determinism (I13): sort all collections, write canonical key order, use a
// stable indented marshaller. The only varying fields are Options.UpdatedAt
// and Options.GeneratedAt — callers wanting bit-equality should pass the
// same values. The function returns the count of repo files written.
func Export(d *sql.DB, opts Options) (int, error) {
	if opts.OutDir == "" {
		return 0, fmt.Errorf("OutDir required")
	}
	repoDir := filepath.Join(opts.OutDir, "repos")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return 0, err
	}

	// Issue #2 I2 (round-trip): export writes JSON for every row in the
	// repositories table — including soft-deleted ones — so that a fresh
	// DB restored from data/ ends up with identical metadata + history.
	// Hard-deleted rows are physically removed; cleanup also drops the file.
	all, err := db.ListAllRepositories(d)
	if err != nil {
		return 0, fmt.Errorf("list all: %w", err)
	}

	// Stable order by (owner, name) for byte-identical re-runs.
	sort.Slice(all, func(i, j int) bool {
		if all[i].Owner != all[j].Owner {
			return all[i].Owner < all[j].Owner
		}
		return all[i].Name < all[j].Name
	})

	var trCache *translate.Cache
	if opts.TranslationCacheDir != "" {
		trCache = &translate.Cache{Dir: opts.TranslationCacheDir}
	}

	written := 0
	for _, r := range all {
		hist, err := db.ListStarHistory(d, r.ID)
		if err != nil {
			return written, fmt.Errorf("history for %s: %w", r.GitHubID, err)
		}
		points := make([]HistoryPoint, len(hist))
		latest := 0
		for i, h := range hist {
			points[i] = HistoryPoint{Date: h.RecordedDate, Stars: h.StarCount}
			if h.RecordedDate <= opts.ComputedDate {
				latest = h.StarCount
			}
		}
		descJA := ""
		if trCache != nil && r.Description != "" {
			if entry, ok, err := trCache.Get(r.Description); err == nil && ok {
				descJA = entry.JA
			}
		}
		detail := RepoDetail{
			RepoID:        r.GitHubID,
			Owner:         r.Owner,
			Name:          r.Name,
			FullName:      r.Owner + "/" + r.Name,
			Description:   r.Description,
			DescriptionJA: descJA,
			URL:           r.URL,
			HomepageURL:   r.HomepageURL,
			Language:      r.Language,
			License:       r.License,
			Topics:        sortedStrings(r.Topics),
			StarCount:     latest,
			ForkCount:     r.ForkCount,
			IsArchived:    r.IsArchived,
			IsFork:        r.IsFork,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			PushedAt:      r.PushedAt,
			DeletedAt:     r.DeletedAt,
			StarHistory:   points,
		}
		if err := writeJSON(filepath.Join(repoDir, r.Owner+"__"+r.Name+".json"), detail); err != nil {
			return written, err
		}
		written++
	}

	// rankings.json — always emit all 6 keys, even if empty (I8).
	rk := Rankings{UpdatedAt: opts.UpdatedAt, Rankings: map[string][]RankingEntry{}}
	for _, key := range AllRankingKeys() {
		period, rankType := splitKey(key)
		rows, err := db.ListRankings(d, period, rankType, opts.ComputedDate)
		if err != nil {
			return written, fmt.Errorf("rankings %s/%s: %w", period, rankType, err)
		}
		entries := make([]RankingEntry, 0, len(rows))
		for _, r := range rows {
			repo, err := getRepoByID(d, r.RepoID)
			if err != nil {
				return written, fmt.Errorf("ranking lookup repo_id=%d: %w", r.RepoID, err)
			}
			descJA := ""
			if trCache != nil && repo.Description != "" {
				if entry, ok, err := trCache.Get(repo.Description); err == nil && ok {
					descJA = entry.JA
				}
			}
			entries = append(entries, RankingEntry{
				Rank:          r.Rank,
				RepoID:        repo.GitHubID,
				Owner:         repo.Owner,
				Name:          repo.Name,
				FullName:      repo.Owner + "/" + repo.Name,
				Description:   repo.Description,
				DescriptionJA: descJA,
				Language:      repo.Language,
				CreatedAt:     repo.CreatedAt,
				StartStars:    r.StartStars,
				EndStars:      r.EndStars,
				StarDelta:     r.StarDelta,
				GrowthPct:     r.GrowthPct,
			})
		}
		rk.Rankings[key] = entries
	}
	if err := writeJSON(filepath.Join(opts.OutDir, "rankings.json"), rk); err != nil {
		return written, err
	}

	// meta.json — counts.
	allRepos, err := db.ListAllRepositories(d)
	if err != nil {
		return written, err
	}
	active, err := db.ListActiveRepositories(d)
	if err != nil {
		return written, err
	}
	meta := Meta{
		GeneratedAt: opts.GeneratedAt,
		TotalRepos:  len(allRepos),
		TotalActive: len(active),
		Periods:     []string{"1d", "7d", "30d"},
		RankTypes:   []string{"breakout", "trending"},
	}
	if err := writeJSON(filepath.Join(opts.OutDir, "meta.json"), meta); err != nil {
		return written, err
	}

	// search-index.json — slim per-repo header search payload (active repos
	// only). Written without indentation: the file is consumed only by the
	// browser, never hand-edited, and dropping pretty-print roughly halves
	// the on-disk and gzipped sizes (~12MB → 6MB raw, ~3MB → 1.5MB gzipped).
	si, err := BuildSearchIndex(d, opts.GeneratedAt, opts.ComputedDate, trCache)
	if err != nil {
		return written, fmt.Errorf("build search index: %w", err)
	}
	if err := writeJSONCompact(filepath.Join(opts.OutDir, "search-index.json"), si); err != nil {
		return written, err
	}

	return written, nil
}

func splitKey(key string) (period, rankType string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '_' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func sortedStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// writeJSONCompact emits minified JSON. Used for machine-only payloads
// (e.g. search-index.json) where pretty-print is wasted bytes.
func writeJSONCompact(path string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func getRepoByID(d *sql.DB, id int64) (db.Repository, error) {
	var ghID string
	if err := d.QueryRow(`SELECT github_id FROM repositories WHERE id=?`, id).Scan(&ghID); err != nil {
		return db.Repository{}, err
	}
	return db.GetRepositoryByGitHubID(d, ghID)
}
