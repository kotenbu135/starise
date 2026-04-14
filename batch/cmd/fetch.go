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

func fetchRepos(client *github.Client, database *sql.DB, targets []string, today string) {
	for _, target := range targets {
		parts := strings.SplitN(target, "/", 2)
		if len(parts) != 2 {
			log.Printf("WARN: skip invalid: %s", target)
			continue
		}
		owner, name := parts[0], parts[1]

		result, err := client.FetchRepo(owner, name)
		if err != nil {
			log.Printf("ERROR: fetch %s: %v", target, err)
			continue
		}

		repo := result.Repo
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
			log.Printf("ERROR: upsert %s: %v", target, err)
			continue
		}

		if err := db.UpsertDailyStar(database, &db.DailyStar{
			RepoID:       repoID,
			RecordedDate: today,
			StarCount:    repo.StargazerCount,
		}); err != nil {
			log.Printf("ERROR: upsert star %s: %v", target, err)
			continue
		}

		log.Printf("OK: %s (%d stars)", target, repo.StargazerCount)
		client.CheckRateLimit(result.RateLimit)
	}
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
