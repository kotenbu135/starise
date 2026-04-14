package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/andygrunwald/go-trending"
	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var discoverMaxPages int

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover repositories via GitHub Search + Trending",
	RunE:  runDiscover,
}

func init() {
	discoverCmd.Flags().IntVar(&discoverMaxPages, "max-pages", 10, "max pages per search query (100 repos/page)")
	rootCmd.AddCommand(discoverCmd)
}

func runDiscover(cmd *cobra.Command, args []string) error {
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

	return discover(client, database, today, discoverMaxPages)
}

func discover(client *github.Client, database *sql.DB, today string, maxPages int) error {
	// Phase 1: GitHub Search API
	log.Println("--- Discover Phase 1: Search API ---")
	searchCount, err := discoverBySearch(client, database, today, maxPages)
	if err != nil {
		log.Printf("WARN: search phase: %v", err)
	}

	// Phase 2: GitHub Trending
	log.Println("--- Discover Phase 2: Trending ---")
	trendCount, err := discoverByTrending(client, database, today)
	if err != nil {
		log.Printf("WARN: trending phase: %v", err)
	}

	log.Printf("Discover complete: search=%d, trending=%d, total=%d",
		searchCount, trendCount, searchCount+trendCount)
	return nil
}

// ── Phase 1: Search API ──

func discoverBySearch(client *github.Client, database *sql.DB, today string, maxPages int) (int, error) {
	now := time.Now()
	recent := now.AddDate(0, -3, 0).Format("2006-01-02")  // 3ヶ月前
	active := now.AddDate(0, -6, 0).Format("2006-01-02")   // 6ヶ月前
	newRepo := now.AddDate(0, 0, -90).Format("2006-01-02") // 90日前

	queries := []string{
		// Star レンジ分割（既存改善）
		"stars:>=10000 fork:false archived:false",
		"stars:5000..9999 fork:false archived:false",
		"stars:1000..4999 fork:false archived:false",
		fmt.Sprintf("stars:100..999 fork:false archived:false pushed:>%s", active),

		// 新規リポ探索（バズ初期捕捉）
		fmt.Sprintf("stars:>50 fork:false archived:false created:>%s", newRepo),
		fmt.Sprintf("stars:10..50 fork:false archived:false created:>%s pushed:>%s", newRepo, recent),

		// 言語別探索（Search API 1000件制限回避）
		fmt.Sprintf("language:Python stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:TypeScript stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:JavaScript stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:Go stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:Rust stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:Java stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:C++ stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:C# stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:Swift stars:>100 fork:false archived:false pushed:>%s", active),
		fmt.Sprintf("language:Kotlin stars:>100 fork:false archived:false pushed:>%s", active),

		// トピック別探索（急成長分野）
		fmt.Sprintf("topic:llm stars:>30 fork:false archived:false pushed:>%s", recent),
		fmt.Sprintf("topic:ai-agent stars:>30 fork:false archived:false pushed:>%s", recent),
		fmt.Sprintf("topic:generative-ai stars:>30 fork:false archived:false pushed:>%s", recent),
		fmt.Sprintf("topic:machine-learning stars:>100 fork:false archived:false pushed:>%s", active),
	}

	totalAdded := 0
	for _, q := range queries {
		count, err := discoverByQuery(client, database, q, today, maxPages)
		if err != nil {
			log.Printf("WARN: query %q: %v", q, err)
			continue
		}
		totalAdded += count
	}
	return totalAdded, nil
}

func discoverByQuery(client *github.Client, database *sql.DB, query, today string, maxPages int) (int, error) {
	var cursor string
	count := 0

	for page := 0; page < maxPages; page++ {
		result, err := client.SearchRepos(query, 100, cursor)
		if err != nil {
			return count, fmt.Errorf("search page %d: %w", page, err)
		}

		log.Printf("Search %q: page %d, %d repos (total: %d)",
			query, page+1, len(result.Repos), result.Total)

		for _, repo := range result.Repos {
			if repo.IsArchived || repo.IsFork || repo.ID == "" {
				continue
			}
			if err := saveDiscoveredRepo(database, repo, today); err != nil {
				log.Printf("ERROR: %s/%s: %v", repo.Owner.Login, repo.Name, err)
			} else {
				count++
			}
		}

		if !result.HasNext {
			break
		}
		cursor = result.EndCursor
		client.CheckRateLimit(result.RateLimit)
		time.Sleep(2 * time.Second)
	}

	return count, nil
}

func saveDiscoveredRepo(database *sql.DB, repo github.RepoData, today string) error {
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
		return fmt.Errorf("upsert: %w", err)
	}

	return db.UpsertDailyStar(database, &db.DailyStar{
		RepoID:       repoID,
		RecordedDate: today,
		StarCount:    repo.StargazerCount,
	})
}

// ── Phase 2: GitHub Trending ──

var trendingLanguages = []string{
	"", // 全言語
	"python", "typescript", "javascript", "go", "rust",
	"java", "c++", "c#", "swift", "kotlin",
}

func discoverByTrending(client *github.Client, database *sql.DB, today string) (int, error) {
	trend := trending.NewTrending()
	totalAdded := 0

	for _, period := range []string{"daily", "weekly", "monthly"} {
		for _, lang := range trendingLanguages {
			projects, err := trend.GetProjects(period, lang)
			if err != nil {
				label := lang
				if label == "" {
					label = "all"
				}
				log.Printf("WARN: trending %s/%s: %v", period, label, err)
				continue
			}

			for _, p := range projects {
				if p.Owner == "" || p.RepositoryName == "" {
					continue
				}

				count, err := saveTrendingRepo(client, database, p, today)
				if err != nil {
					log.Printf("ERROR: trending %s/%s: %v", p.Owner, p.RepositoryName, err)
					continue
				}
				totalAdded += count
			}

			time.Sleep(1 * time.Second) // trending ページ間隔
		}
	}

	log.Printf("Trending: %d repos added/updated", totalAdded)
	return totalAdded, nil
}

func saveTrendingRepo(ghClient *github.Client, database *sql.DB, p trending.Project, today string) (int, error) {
	// trending からは星数やID取れない → GraphQL で個別取得
	result, err := ghClient.FetchRepo(p.Owner, p.RepositoryName)
	if err != nil {
		return 0, err
	}

	repo := result.Repo
	if repo.IsArchived || repo.IsFork {
		return 0, nil
	}

	if err := saveDiscoveredRepo(database, repo, today); err != nil {
		return 0, err
	}

	ghClient.CheckRateLimit(result.RateLimit)
	return 1, nil
}
