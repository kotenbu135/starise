package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var runSeedFile string
var runOutDir string
var runMaxPages int
var runSkipDiscover bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "All-in-one: discover + fetch + compute + export",
	RunE:  runAll,
}

func init() {
	runCmd.Flags().StringVar(&runSeedFile, "seed-file", "seeds.txt", "seed repos file")
	runCmd.Flags().StringVar(&runOutDir, "out-dir", "../data", "output directory for JSON files")
	runCmd.Flags().IntVar(&runMaxPages, "max-pages", 10, "max pages per discover query (100 repos/page)")
	runCmd.Flags().BoolVar(&runSkipDiscover, "skip-discover", false, "skip discover phase")
	rootCmd.AddCommand(runCmd)
}

func runAll(cmd *cobra.Command, args []string) error {
	seeds, err := readSeeds(runSeedFile)
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

	// 1. Discover
	if !runSkipDiscover {
		log.Println("=== Phase 1: Discover ===")
		if err := discover(client, database, today, runMaxPages); err != nil {
			log.Printf("WARN: discover: %v", err)
		}
	}

	// 2. Fetch (seeds + unfetched DB repos)
	log.Println("=== Phase 2: Fetch ===")
	targets := mergeTargets(seeds, database, today)
	fetchRepos(client, database, targets, today)

	// 3. Compute
	log.Println("=== Phase 3: Compute ===")
	if err := ranking.Compute(database); err != nil {
		return err
	}

	// 4. Export
	log.Println("=== Phase 4: Export ===")
	return export.Export(database, runOutDir)
}
