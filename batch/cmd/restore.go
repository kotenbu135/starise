package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/restore"
)

var restoreInDir string

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Rebuild DB from data/repos/*.json (source-of-truth recovery)",
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().StringVar(&restoreInDir, "in-dir", "../data", "data directory containing repos/*.json")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(_ *cobra.Command, _ []string) error {
	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		return err
	}

	stats, err := restore.FromDir(d, restoreInDir)
	if err != nil {
		return err
	}
	log.Printf("restore: %+v", stats)
	return nil
}
