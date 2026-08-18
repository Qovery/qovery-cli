package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var clusterOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Manage the Qovery Operator on a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	clusterCmd.AddCommand(clusterOperatorCmd)
}
