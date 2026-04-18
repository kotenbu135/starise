package cmd

import "github.com/spf13/cobra"

var (
	runSeedFile     string
	runOutDir       string
	runRestoreFrom  string
	runTopN         int
	runMaxPages     int
	runSkipDiscover bool
	runSkipRefresh  bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "All-in-one: restore -> fetch -> discover -> refresh -> compute -> export",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&runSeedFile, "seed-file", "seeds.txt", "seed list path")
	runCmd.Flags().StringVar(&runOutDir, "out-dir", "../data", "output directory")
	runCmd.Flags().StringVar(&runRestoreFrom, "restore-from", "../data", "restore source dir (empty = skip)")
	runCmd.Flags().IntVar(&runTopN, "top-n", 2000, "max rank entries per slot")
	runCmd.Flags().IntVar(&runMaxPages, "max-pages", 10, "discover search pages")
	runCmd.Flags().BoolVar(&runSkipDiscover, "skip-discover", false, "skip discover step")
	runCmd.Flags().BoolVar(&runSkipRefresh, "skip-refresh", false, "skip refresh step")
}
