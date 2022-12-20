package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var databaseCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a database deployment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		utils.PrintlnInfo("Use: 'qovery environment cancel' to cancel this deployment")
	},
}

func init() {
	databaseCmd.AddCommand(databaseCancelCmd)
}
