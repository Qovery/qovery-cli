package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage application environment variables and secrets",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationEnvCmd)
}
