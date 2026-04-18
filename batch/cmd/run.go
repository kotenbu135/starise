package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/fetch"
	"github.com/kotenbu135/starise/batch/internal/github"
	"github.com/kotenbu135/starise/batch/internal/pipeline"
)

var (
	runSeedFile     string
	runOutDir       string
	runTopN         int
	runMaxPages     int
	runSkipDiscover bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Fetch + discover + compute + export in one invocation",
	RunE:  runAll,
}

func init() {
	runCmd.Flags().StringVar(&runSeedFile, "seed-file", "seeds.txt", "seed list")
	runCmd.Flags().StringVar(&runOutDir, "out-dir", "../data", "output directory")
	runCmd.Flags().IntVar(&runTopN, "top-n", 500, "max entries per period (<=0 = all)")
	runCmd.Flags().IntVar(&runMaxPages, "max-pages", 10, "pages per discover query")
	runCmd.Flags().BoolVar(&runSkipDiscover, "skip-discover", false, "skip search-based discovery")
	rootCmd.AddCommand(runCmd)
}

func runAll(_ *cobra.Command, _ []string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}
	body, err := os.ReadFile(runSeedFile)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	seeds, err := fetch.ParseSeedsText(string(body))
	if err != nil {
		return err
	}

	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC()
	return pipeline.RunAll(d, pipeline.RunOptions{
		Client:       github.NewAPIClient(token),
		Seeds:        seeds,
		Today:        now.Format("2006-01-02"),
		UpdatedAt:    now.Format(time.RFC3339),
		ComputedDate: now.Format("2006-01-02"),
		OutDir:       runOutDir,
		TopN:         runTopN,
		SkipDiscover: runSkipDiscover,
		MaxPages:     runMaxPages,
	})
}
