package cmd

import "github.com/spf13/cobra"

var (
	exportOutDir string
	exportTopN   int
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all non-deleted repos + rankings + meta JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportOutDir, "out-dir", "../data", "output directory")
	exportCmd.Flags().IntVar(&exportTopN, "top-n", 2000, "max rank entries per slot")
}
