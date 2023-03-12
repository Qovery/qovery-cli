package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationEnvOverrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Manage application environment variable and secret overrides",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	applicationEnvCmd.AddCommand(applicationEnvOverrideCmd)
}
