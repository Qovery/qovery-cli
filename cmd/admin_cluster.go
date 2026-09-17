package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminClusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
	// TODO (mzo) add parameter to random deploy clusters
	// TODO (mzo) be able to handle upgrades of STOPPED clusters, to automatically upgrade & stop them
	// TODO (mzo) handle pending clusters queue, when clusters couldn't be deployed because not in a final state
	// TODO (mzo) handle progression in a file to let resume from the last deployment launched in case of interruption
)

func init() {
	adminCmd.AddCommand(adminClusterCmd)
}
