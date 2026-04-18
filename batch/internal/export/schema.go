// Package export writes the canonical JSON artefacts under data/.
//
// Files produced:
//   - data/repos/{owner}__{name}.json — RepoDetail per non-deleted repo
//   - data/rankings.json              — 6-slot Rankings
//   - data/meta.json                  — Meta summary
//
// Per issue #2 invariant I13, the writer must be deterministic given a fixed
// computed_date and DB state (modulo the timestamp fields enumerated below).
package export

// HistoryPoint is one row of a repo's daily star history.
type HistoryPoint struct {
	Date  string `json:"date"`
	Stars int    `json:"stars"`
}

// RepoDetail is the per-repo JSON document under data/repos/.
type RepoDetail struct {
	RepoID       string         `json:"repo_id"`
	Owner        string         `json:"owner"`
	Name         string         `json:"name"`
	FullName     string         `json:"full_name"`
	Description  string         `json:"description"`
	URL          string         `json:"url"`
	HomepageURL  string         `json:"homepage_url"`
	Language     string         `json:"language"`
	License      string         `json:"license"`
	Topics       []string       `json:"topics"`
	StarCount    int            `json:"star_count"`
	ForkCount    int            `json:"fork_count"`
	IsArchived   bool           `json:"is_archived"`
	IsFork       bool           `json:"is_fork"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	PushedAt     string         `json:"pushed_at"`
	DeletedAt    string         `json:"deleted_at"`
	StarHistory  []HistoryPoint `json:"star_history"`
}

// RankingEntry is one row inside rankings.json.
type RankingEntry struct {
	Rank       int     `json:"rank"`
	RepoID     string  `json:"repo_id"`
	Owner      string  `json:"owner"`
	Name       string  `json:"name"`
	FullName   string  `json:"full_name"`
	Language   string  `json:"language"`
	StartStars int     `json:"start_stars"`
	EndStars   int     `json:"end_stars"`
	StarDelta  int     `json:"star_delta"`
	GrowthPct  float64 `json:"growth_pct"`
}

// Rankings is the rankings.json document. UpdatedAt is the only field
// that is allowed to vary between byte-identical runs (see I13).
//
// Rankings map keys: "1d_breakout", "1d_trending", "7d_breakout",
// "7d_trending", "30d_breakout", "30d_trending".
type Rankings struct {
	UpdatedAt string                    `json:"updated_at"`
	Rankings  map[string][]RankingEntry `json:"rankings"`
}

// Meta is the meta.json document.
type Meta struct {
	GeneratedAt string   `json:"generated_at"`
	TotalRepos  int      `json:"total_repos"`
	TotalActive int      `json:"total_active"`
	Periods     []string `json:"periods"`
	RankTypes   []string `json:"rank_types"`
}

// AllRankingKeys returns the canonical six slot keys in deterministic order.
func AllRankingKeys() []string {
	return []string{
		"1d_breakout", "1d_trending",
		"7d_breakout", "7d_trending",
		"30d_breakout", "30d_trending",
	}
}
