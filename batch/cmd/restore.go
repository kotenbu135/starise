package cmd

import "github.com/spf13/cobra"

var restoreInDir string

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore DB from data/repos/*.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreInDir, "in-dir", "../data", "input directory")
}
