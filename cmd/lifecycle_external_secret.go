package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleExternalSecretCmd = &cobra.Command{
	Use:   "external-secret",
	Short: "Manage lifecycle external secrets",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleExternalSecretCmd)
}
