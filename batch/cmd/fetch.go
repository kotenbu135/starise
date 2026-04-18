package cmd

import "github.com/spf13/cobra"

var fetchSeedFile string

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch seed repositories and today snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	fetchCmd.Flags().StringVar(&fetchSeedFile, "seed-file", "seeds.txt", "seed list path")
}
