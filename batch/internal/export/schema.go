// Package export writes the batch's SQLite state to a deterministic set of
// JSON files consumed by the frontend. The types in this file form the public
// contract: any change here is a breaking change for web/.
package export

// Rankings is the shape of data/rankings.json.
type Rankings struct {
	UpdatedAt string                    `json:"updated_at"`
	Rankings  map[string][]RankingEntry `json:"rankings"`
}

// RankingEntry is one row in a ranking list.
type RankingEntry struct {
	Rank       int     `json:"rank"`
	RepoID     string  `json:"repo_id"`
	Owner      string  `json:"owner"`
	Name       string  `json:"name"`
	FullName   string  `json:"full_name"`
	Language   string  `json:"language,omitempty"`
	StartStars int     `json:"start_stars"`
	EndStars   int     `json:"end_stars"`
	StarDelta  int     `json:"star_delta"`
	GrowthPct  float64 `json:"growth_pct"`
}

// Meta is data/meta.json.
type Meta struct {
	GeneratedAt string   `json:"generated_at"`
	TotalRepos  int      `json:"total_repos"`
	Periods     []string `json:"periods"`
}

// RepoDetail is data/repos/{owner}__{name}.json.
type RepoDetail struct {
	RepoID      string      `json:"repo_id"`
	Owner       string      `json:"owner"`
	Name        string      `json:"name"`
	FullName    string      `json:"full_name"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	HomepageURL string      `json:"homepage_url,omitempty"`
	Language    string      `json:"language,omitempty"`
	License     string      `json:"license,omitempty"`
	Topics      []string    `json:"topics"`
	StarCount   int         `json:"star_count"`
	ForkCount   int         `json:"fork_count"`
	StarHistory []StarPoint `json:"star_history"`
}

// StarPoint is one point on a repo's star-history series.
type StarPoint struct {
	Date  string `json:"date"`
	Stars int    `json:"stars"`
}
