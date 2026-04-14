package cmd

import (
	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

var outDir string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export rankings and repo data as static JSON",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVar(&outDir, "out-dir", "../data", "output directory for JSON files")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	return export.Export(database, outDir)
}
