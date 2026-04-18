package cmd

import "github.com/spf13/cobra"

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh today snapshot for all non-deleted repos via bulk nodes()",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
