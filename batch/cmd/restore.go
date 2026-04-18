package cmd

import (
	"fmt"

	"github.com/kotenbu135/starise/batch/internal/restore"
	"github.com/spf13/cobra"
)

var restoreInDir string

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore DB from data/repos/*.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		res, err := restore.FromDir(d, restoreInDir)
		if err != nil {
			return err
		}
		fmt.Printf("restore: repos=%d snapshots=%d\n", res.Repos, res.Snapshots)
		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreInDir, "in-dir", "../data", "input directory")
}
