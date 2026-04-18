package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/pipeline"
)

var computeCmd = &cobra.Command{
	Use:   "compute",
	Short: "Compute 1d/7d/30d growth rankings",
	RunE:  runCompute,
}

func init() {
	rootCmd.AddCommand(computeCmd)
}

func runCompute(_ *cobra.Command, _ []string) error {
	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		return err
	}
	today := time.Now().UTC().Format("2006-01-02")
	return pipeline.Compute(d, today)
}
