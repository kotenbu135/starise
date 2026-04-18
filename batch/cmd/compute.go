package cmd

import "github.com/spf13/cobra"

var computeCmd = &cobra.Command{
	Use:   "compute",
	Short: "Compute 2-axis rankings (breakout + trending) for 1d/7d/30d",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
