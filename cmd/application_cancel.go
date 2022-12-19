package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel an application deployment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		utils.PrintlnInfo("Use: 'qovery environment cancel' to cancel this deployment")
	},
}

func init() {
	applicationCmd.AddCommand(applicationCancelCmd)
}
