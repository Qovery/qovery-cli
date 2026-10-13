package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var adminClusterOperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Manage the Qovery Operator fleet",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
		os.Exit(0)
	},
}

func init() {
	adminClusterCmd.AddCommand(adminClusterOperatorCmd)
}
