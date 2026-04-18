package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
)

type Meta struct {
	GeneratedAt string   `json:"generated_at"`
	TotalRepos  int      `json:"total_repos"`
	Periods     []string `json:"periods"`
}

type RankingEntry struct {
	Rank        int     `json:"rank"`
	Owner       string  `json:"owner"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Language    string  `json:"language"`
	License     string  `json:"license"`
	StarCount   int     `json:"star_count"`
	StarDelta   int     `json:"star_delta"`
	GrowthRate  float64 `json:"growth_rate"`
	URL         string  `json:"url"`
	CreatedAt   string  `json:"created_at"`
}

type RankingsFile struct {
	UpdatedAt string                    `json:"updated_at"`
	Rankings  map[string][]RankingEntry `json:"rankings"`
}

type RepoDetail struct {
	Owner       string          `json:"owner"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	HomepageURL string          `json:"homepage_url"`
	Language    string          `json:"language"`
	License     string          `json:"license"`
	Topics      json.RawMessage `json:"topics"`
	ForkCount   int             `json:"fork_count"`
	StarCount   int             `json:"star_count"`
	IsArchived  bool            `json:"is_archived"`
	StarHistory []StarPoint     `json:"star_history"`
}

type StarPoint struct {
	Date  string `json:"date"`
	Stars int    `json:"stars"`
}

func Export(database *sql.DB, outDir string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	repos, err := db.GetAllRepositories(database)
	if err != nil {
		return fmt.Errorf("get repos: %w", err)
	}

	// rankings.json
	rankingsFile := RankingsFile{
		UpdatedAt: now,
		Rankings:  make(map[string][]RankingEntry),
	}

	for _, period := range []string{"1d", "7d", "30d"} {
		entries, err := getRankingEntries(database, period, repos)
		if err != nil {
			return err
		}
		rankingsFile.Rankings[period] = entries
	}

	if err := writeJSON(filepath.Join(outDir, "rankings.json"), rankingsFile); err != nil {
		return err
	}

	// repos/{owner}__{name}.json
	reposDir := filepath.Join(outDir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		return fmt.Errorf("mkdir repos: %w", err)
	}

	// Build set of files this export run will produce; delete everything else.
	// Prevents orphan accumulation when repos are archived / renamed / removed.
	expected := make(map[string]struct{}, len(repos))
	for _, r := range repos {
		expected[fmt.Sprintf("%s__%s.json", r.Owner, r.Name)] = struct{}{}
	}
	if existing, err := os.ReadDir(reposDir); err == nil {
		removed := 0
		for _, de := range existing {
			if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
				continue
			}
			if _, ok := expected[de.Name()]; ok {
				continue
			}
			if err := os.Remove(filepath.Join(reposDir, de.Name())); err != nil {
				log.Printf("WARN: remove orphan %s: %v", de.Name(), err)
				continue
			}
			removed++
		}
		if removed > 0 {
			log.Printf("Removed %d orphan repo JSON files", removed)
		}
	}

	latestStars := getLatestStars(database, repos)

	for _, r := range repos {
		history, err := db.GetStarHistory(database, r.ID)
		if err != nil {
			return fmt.Errorf("get star history: %w", err)
		}

		starPoints := make([]StarPoint, len(history))
		for i, h := range history {
			starPoints[i] = StarPoint{Date: h.Date, Stars: h.Stars}
		}

		detail := RepoDetail{
			Owner:       r.Owner,
			Name:        r.Name,
			Description: r.Description,
			URL:         r.URL,
			HomepageURL: r.HomepageURL,
			Language:    r.Language,
			License:     r.License,
			Topics:      json.RawMessage(r.Topics),
			ForkCount:   r.ForkCount,
			StarCount:   latestStars[r.ID],
			IsArchived:  r.IsArchived,
			StarHistory: starPoints,
		}

		fname := fmt.Sprintf("%s__%s.json", r.Owner, r.Name)
		if err := writeJSON(filepath.Join(reposDir, fname), detail); err != nil {
			return err
		}
	}

	// meta.json
	meta := Meta{
		GeneratedAt: now,
		TotalRepos:  len(repos),
		Periods:     []string{"1d", "7d", "30d"},
	}
	if err := writeJSON(filepath.Join(outDir, "meta.json"), meta); err != nil {
		return err
	}

	log.Printf("Exported %d repos to %s", len(repos), outDir)
	return nil
}

func getRankingEntries(database *sql.DB, period string, repos []db.Repository) ([]RankingEntry, error) {
	repoMap := make(map[int64]db.Repository, len(repos))
	for _, r := range repos {
		repoMap[r.ID] = r
	}

	rows, err := database.Query(`
		SELECT repo_id, rank, star_end, star_delta, growth_rate
		FROM rankings
		WHERE period = ? AND computed_date = (SELECT MAX(computed_date) FROM rankings WHERE period = ?)
		ORDER BY rank`, period, period)
	if err != nil {
		return nil, fmt.Errorf("query rankings (%s): %w", period, err)
	}
	defer rows.Close()

	entries := make([]RankingEntry, 0)
	for rows.Next() {
		var repoID int64
		var e RankingEntry
		if err := rows.Scan(&repoID, &e.Rank, &e.StarCount, &e.StarDelta, &e.GrowthRate); err != nil {
			return nil, fmt.Errorf("scan ranking: %w", err)
		}
		if r, ok := repoMap[repoID]; ok {
			e.Owner = r.Owner
			e.Name = r.Name
			e.Description = r.Description
			e.Language = r.Language
			e.License = r.License
			e.URL = r.URL
			e.CreatedAt = r.CreatedAt
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func getLatestStars(database *sql.DB, repos []db.Repository) map[int64]int {
	m := make(map[int64]int, len(repos))
	for _, r := range repos {
		row := database.QueryRow(`
			SELECT star_count FROM daily_stars
			WHERE repo_id = ? ORDER BY recorded_date DESC LIMIT 1`, r.ID)
		var count int
		if row.Scan(&count) == nil {
			m[r.ID] = count
		}
	}
	return m
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
