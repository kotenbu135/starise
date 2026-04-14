package cmd

import (
	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var computeCmd = &cobra.Command{
	Use:   "compute",
	Short: "Calculate 7d/30d star growth rankings",
	RunE:  runCompute,
}

func init() {
	rootCmd.AddCommand(computeCmd)
}

func runCompute(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	return ranking.Compute(database)
}
