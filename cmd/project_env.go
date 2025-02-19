package cmd

import (
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage project variables and secrets",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	projectCmd.AddCommand(projectEnvCmd)
}
