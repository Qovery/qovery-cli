package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var cronjobEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage cronjob environment variables and secrets",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobEnvCmd)
}
