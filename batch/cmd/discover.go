package cmd

import "github.com/spf13/cobra"

var discoverMaxPages int

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover new repositories via Search API",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	discoverCmd.Flags().IntVar(&discoverMaxPages, "max-pages", 10, "max search pages")
}
