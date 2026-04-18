package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
)

var (
	exportOutDir string
	exportTopN   int
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export rankings and repo details as static JSON",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportOutDir, "out-dir", "../data", "output directory")
	exportCmd.Flags().IntVar(&exportTopN, "top-n", 500, "max entries per period (<=0 = all)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(_ *cobra.Command, _ []string) error {
	d, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		return err
	}
	now := time.Now().UTC()
	return export.Export(
		d,
		exportOutDir,
		now.Format(time.RFC3339),
		now.Format("2006-01-02"),
		exportTopN,
	)
}
