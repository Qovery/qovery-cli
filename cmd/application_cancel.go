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

		// TODO make app cancel working and add --watch arg
		// TODO provide a way to cancel a deployment per service
	},
}

func init() {
	applicationCmd.AddCommand(applicationCancelCmd)
}
