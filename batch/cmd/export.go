package cmd

import (
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/spf13/cobra"
)

var (
	exportOutDir string
	exportTopN   int
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all non-deleted repos + rankings + meta JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()

		t := today()
		if t == "" {
			t = time.Now().UTC().Format("2006-01-02")
		}
		now := time.Now().UTC().Format(time.RFC3339)

		written, err := export.Export(d, export.Options{
			OutDir: exportOutDir, UpdatedAt: now, GeneratedAt: now,
			ComputedDate: t, TopN: exportTopN,
		})
		if err != nil {
			return err
		}
		cl, err := export.Cleanup(d, exportOutDir, t)
		if err != nil {
			return err
		}
		fmt.Printf("export: wrote=%d cleanup_orphans=%d hard_deleted=%d\n",
			written, cl.OrphansRemoved, cl.HardDeleted)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportOutDir, "out-dir", "../data", "output directory")
	exportCmd.Flags().IntVar(&exportTopN, "top-n", 2000, "max rank entries per slot")
}
