package cmd

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var seedFile string

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch repository data and star counts from GitHub",
	RunE:  runFetch,
}

func init() {
	fetchCmd.Flags().StringVar(&seedFile, "seed-file", "seeds.txt", "seed repos file (owner/name per line)")
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(cmd *cobra.Command, args []string) error {
	seeds, err := readSeeds(seedFile)
	if err != nil {
		return err
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	client := github.NewClient(token)
	today := time.Now().UTC().Format("2006-01-02")

	targets := mergeTargets(seeds, database, today)
	fetchRepos(client, database, targets, today)
	return nil
}

const batchSize = 20

func fetchRepos(client *github.Client, database *sql.DB, targets []string, today string) {
	for i := 0; i < len(targets); i += batchSize {
		end := i + batchSize
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[i:end]

		result, err := client.FetchReposBatch(batch)
		if err != nil {
			log.Printf("ERROR: batch fetch [%d..%d]: %v, falling back to single", i, end-1, err)
			fetchReposSingle(client, database, batch, today)
			continue
		}

		for _, slug := range batch {
			repo, ok := result.Repos[slug]
			if !ok {
				log.Printf("WARN: batch miss %s", slug)
				continue
			}
			saveRepo(database, slug, repo, today)
		}

		log.Printf("Batch [%d..%d]: %d/%d OK", i, end-1, len(result.Repos), len(batch))
		client.CheckRateLimit(result.RateLimit)
	}
}

// fetchReposSingle is the fallback for failed batches.
func fetchReposSingle(client *github.Client, database *sql.DB, targets []string, today string) {
	for _, target := range targets {
		parts := strings.SplitN(target, "/", 2)
		if len(parts) != 2 {
			log.Printf("WARN: skip invalid: %s", target)
			continue
		}

		result, err := client.FetchRepo(parts[0], parts[1])
		if err != nil {
			log.Printf("ERROR: fetch %s: %v", target, err)
			continue
		}

		saveRepo(database, target, result.Repo, today)
		client.CheckRateLimit(result.RateLimit)
	}
}

func saveRepo(database *sql.DB, slug string, repo github.RepoData, today string) {
	topics, _ := json.Marshal(extractTopics(repo))

	r := &db.Repository{
		GitHubID:    repo.ID,
		Owner:       repo.Owner.Login,
		Name:        repo.Name,
		Description: deref(repo.Description),
		URL:         repo.URL,
		HomepageURL: deref(repo.HomepageURL),
		Language:    repoLanguage(repo),
		License:     repoLicense(repo),
		Topics:      string(topics),
		IsArchived:  repo.IsArchived,
		IsFork:      repo.IsFork,
		ForkCount:   repo.ForkCount,
		CreatedAt:   repo.CreatedAt,
		UpdatedAt:   repo.UpdatedAt,
		PushedAt:    repo.PushedAt,
	}

	repoID, err := db.UpsertRepository(database, r)
	if err != nil {
		log.Printf("ERROR: upsert %s: %v", slug, err)
		return
	}

	if err := db.UpsertDailyStar(database, &db.DailyStar{
		RepoID:       repoID,
		RecordedDate: today,
		StarCount:    repo.StargazerCount,
	}); err != nil {
		log.Printf("ERROR: upsert star %s: %v", slug, err)
		return
	}

	log.Printf("OK: %s (%d stars)", slug, repo.StargazerCount)
}

func mergeTargets(seeds []string, database *sql.DB, today string) []string {
	seen := make(map[string]bool, len(seeds))
	targets := make([]string, 0, len(seeds))

	for _, s := range seeds {
		seen[s] = true
		targets = append(targets, s)
	}

	missing, err := db.GetReposNotFetchedToday(database, today)
	if err != nil {
		log.Printf("WARN: query unfetched repos: %v", err)
		return targets
	}

	for _, slug := range missing {
		if !seen[slug] {
			targets = append(targets, slug)
			seen[slug] = true
		}
	}

	log.Printf("Fetch targets: %d seeds + %d DB repos = %d total",
		len(seeds), len(targets)-len(seeds), len(targets))
	return targets
}

func readSeeds(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open seeds: %w", err)
	}
	defer f.Close()

	var seeds []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		seeds = append(seeds, line)
	}
	return seeds, sc.Err()
}

func extractTopics(r github.RepoData) []string {
	topics := make([]string, 0, len(r.RepositoryTopics.Nodes))
	for _, n := range r.RepositoryTopics.Nodes {
		topics = append(topics, n.Topic.Name)
	}
	return topics
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func repoLanguage(r github.RepoData) string {
	if r.PrimaryLanguage == nil {
		return ""
	}
	return r.PrimaryLanguage.Name
}

func repoLicense(r github.RepoData) string {
	if r.LicenseInfo == nil {
		return ""
	}
	return r.LicenseInfo.SpdxID
}
