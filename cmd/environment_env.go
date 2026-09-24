package cmd

import (
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables and secrets",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentEnvCmd)
}
