package cmd

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
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var restoreInDir string

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore DB state from data/repos/*.json (source of truth)",
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().StringVar(&restoreInDir, "in-dir", "../data", "input directory containing repos/ and rankings.json")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	return Restore(database, restoreInDir)
}

type restoreRepoFile struct {
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
	StarHistory []struct {
		Date  string `json:"date"`
		Stars int    `json:"stars"`
	} `json:"star_history"`
}

// Restore rebuilds the DB from data/repos/*.json files.
// This makes data/ the source of truth — independent of any ephemeral cache.
func Restore(database *sql.DB, inDir string) error {
	reposDir := filepath.Join(inDir, "repos")
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Restore: no %s, starting empty", reposDir)
			return nil
		}
		return fmt.Errorf("read repos dir: %w", err)
	}

	start := time.Now()

	// Batch into a single transaction — 40k+ auto-commits take ~90s; one tx ~5s.
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	repoStmt, err := tx.Prepare(`
		INSERT INTO repositories (github_id, owner, name, description, url, homepage_url,
			language, license, topics, is_archived, is_fork, fork_count,
			created_at, updated_at, pushed_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(github_id) DO UPDATE SET
			owner=excluded.owner, name=excluded.name, description=excluded.description,
			url=excluded.url, homepage_url=excluded.homepage_url,
			language=excluded.language, license=excluded.license, topics=excluded.topics,
			is_archived=excluded.is_archived, fork_count=excluded.fork_count
		RETURNING id`)
	if err != nil {
		return fmt.Errorf("prepare repo stmt: %w", err)
	}
	defer repoStmt.Close()

	starStmt, err := tx.Prepare(`
		INSERT INTO daily_stars (repo_id, recorded_date, star_count)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_id, recorded_date) DO UPDATE SET star_count=excluded.star_count`)
	if err != nil {
		return fmt.Errorf("prepare star stmt: %w", err)
	}
	defer starStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	repoCount := 0
	starCount := 0

	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}

		path := filepath.Join(reposDir, de.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("WARN: read %s: %v", path, err)
			continue
		}

		var rf restoreRepoFile
		if err := json.Unmarshal(data, &rf); err != nil {
			log.Printf("WARN: parse %s: %v", path, err)
			continue
		}

		if rf.Owner == "" || rf.Name == "" {
			continue
		}

		topics := string(rf.Topics)
		if topics == "" {
			topics = "[]"
		}

		// github_id is not stored in JSON exports; synthesize from owner/name so
		// UNIQUE(github_id) conflict still works. Real fetches overwrite with
		// GitHub's node id on subsequent upsert (via owner/name UNIQUE index).
		githubID := "restored:" + rf.Owner + "/" + rf.Name

		var repoID int64
		if err := repoStmt.QueryRow(
			githubID, rf.Owner, rf.Name, rf.Description, rf.URL, rf.HomepageURL,
			rf.Language, rf.License, topics, rf.IsArchived, false, rf.ForkCount,
			"", "", "", now,
		).Scan(&repoID); err != nil {
			log.Printf("ERROR: upsert %s/%s: %v", rf.Owner, rf.Name, err)
			continue
		}
		repoCount++

		for _, h := range rf.StarHistory {
			if h.Date == "" {
				continue
			}
			if _, err := starStmt.Exec(repoID, h.Date, h.Stars); err != nil {
				log.Printf("WARN: upsert star %s/%s @ %s: %v", rf.Owner, rf.Name, h.Date, err)
				continue
			}
			starCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	log.Printf("Restore: %d repos, %d daily_stars in %v", repoCount, starCount, time.Since(start))
	return nil
}
