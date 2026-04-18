package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kotenbu135/starise/batch/internal/db"
)

// Periods is the ordered set of periods written to rankings.json.
var Periods = []string{"1d", "7d", "30d"}

// Export regenerates the full data/ tree under outDir. All writes go to
// temp files that are renamed in place, so readers never see torn files.
//
//   - updatedAt:    ISO-8601 string for updated_at / generated_at
//   - computedDate: YYYY-MM-DD — rankings row key
//   - topN:         cap on entries per period (>0; <=0 means all)
func Export(d *sql.DB, outDir, updatedAt, computedDate string, topN int) error {
	if err := os.MkdirAll(filepath.Join(outDir, "repos"), 0o755); err != nil {
		return fmt.Errorf("mkdir repos: %w", err)
	}

	repos, err := db.ListRepositories(d)
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}
	reposByID := make(map[int64]db.Repository, len(repos))
	for _, r := range repos {
		reposByID[r.ID] = r
	}

	// rankings.json
	rankings, rankedIDs, err := buildRankings(d, reposByID, computedDate, topN)
	if err != nil {
		return fmt.Errorf("build rankings: %w", err)
	}
	rankings.UpdatedAt = updatedAt
	if err := writeJSONAtomic(filepath.Join(outDir, "rankings.json"), rankings); err != nil {
		return err
	}

	// meta.json
	meta := Meta{
		GeneratedAt: updatedAt,
		TotalRepos:  len(repos),
		Periods:     append([]string(nil), Periods...),
	}
	if err := writeJSONAtomic(filepath.Join(outDir, "meta.json"), meta); err != nil {
		return err
	}

	// repos/{owner}__{name}.json — only for repos that appear in at least one ranking.
	for id := range rankedIDs {
		r, ok := reposByID[id]
		if !ok {
			continue
		}
		detail, err := buildRepoDetail(d, r)
		if err != nil {
			return fmt.Errorf("detail %d: %w", id, err)
		}
		path := filepath.Join(outDir, "repos", fmt.Sprintf("%s__%s.json", r.Owner, r.Name))
		if err := writeJSONAtomic(path, detail); err != nil {
			return err
		}
	}
	return nil
}

func buildRankings(d *sql.DB, reposByID map[int64]db.Repository, date string, topN int) (Rankings, map[int64]struct{}, error) {
	out := Rankings{Rankings: make(map[string][]RankingEntry, len(Periods))}
	ranked := make(map[int64]struct{})
	for _, p := range Periods {
		rows, err := db.ListRankings(d, p, date, topN)
		if err != nil {
			return Rankings{}, nil, err
		}
		entries := make([]RankingEntry, 0, len(rows))
		for _, r := range rows {
			repo, ok := reposByID[r.RepoID]
			if !ok {
				continue
			}
			entries = append(entries, RankingEntry{
				Rank:       r.Rank,
				RepoID:     repo.GitHubID,
				Owner:      repo.Owner,
				Name:       repo.Name,
				FullName:   repo.Owner + "/" + repo.Name,
				Language:   repo.Language,
				StartStars: r.StartStars,
				EndStars:   r.EndStars,
				StarDelta:  r.StarDelta,
				GrowthPct:  r.GrowthPct,
			})
			ranked[r.RepoID] = struct{}{}
		}
		out.Rankings[p] = entries
	}
	return out, ranked, nil
}

func buildRepoDetail(d *sql.DB, r db.Repository) (RepoDetail, error) {
	snaps, err := db.ListDailyStars(d, r.ID)
	if err != nil {
		return RepoDetail{}, err
	}
	history := make([]StarPoint, 0, len(snaps))
	for _, s := range snaps {
		history = append(history, StarPoint{Date: s.RecordedDate, Stars: s.StarCount})
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Date < history[j].Date })

	topics := []string{}
	if r.Topics != "" {
		_ = json.Unmarshal([]byte(r.Topics), &topics)
	}

	latest := 0
	if n := len(snaps); n > 0 {
		latest = snaps[n-1].StarCount
	}
	return RepoDetail{
		RepoID:      r.GitHubID,
		Owner:       r.Owner,
		Name:        r.Name,
		FullName:    r.Owner + "/" + r.Name,
		Description: r.Description,
		URL:         r.URL,
		HomepageURL: r.HomepageURL,
		Language:    r.Language,
		License:     r.License,
		Topics:      topics,
		StarCount:   latest,
		ForkCount:   r.ForkCount,
		StarHistory: history,
	}, nil
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
