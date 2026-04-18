package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/kotenbu135/starise/batch/internal/refresh"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh today's snapshot for all non-deleted repos via bulk nodes()",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		c, err := newClient()
		if err != nil {
			return err
		}
		t := today()
		if t == "" {
			t = time.Now().UTC().Format("2006-01-02")
		}
		res, err := refresh.Run(context.Background(), d, c, t, refresh.DefaultMaxFailureRate)
		fmt.Printf("refresh: refreshed=%d soft_deleted=%d archived_flips=%d failure_rate=%.2f%%\n",
			res.Refreshed, res.SoftDeleted, res.ArchivedFlipped, res.FailureRate*100)
		return err
	},
}
