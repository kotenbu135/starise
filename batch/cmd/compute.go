package cmd

import (
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/ranking"
	"github.com/spf13/cobra"
)

var computeTopN int

var computeCmd = &cobra.Command{
	Use:   "compute",
	Short: "Compute 2-axis rankings (breakout + trending) for 1d/7d/30d",
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
		if err := ranking.ComputeAndCheck(d, t, computeTopN); err != nil {
			return err
		}
		fmt.Printf("compute: ok (date=%s, topN=%d)\n", t, computeTopN)
		return nil
	},
}

func init() {
	computeCmd.Flags().IntVar(&computeTopN, "top-n", 2000, "max ranking entries per slot")
}
