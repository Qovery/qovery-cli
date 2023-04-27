package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var targetEnvironmentName string
var newEnvironmentName string
var clusterName string
var environmentType string
var applyDeploymentRule bool

var environmentCmd = &cobra.Command{
	Use:   "environment",
	Short: "Manage environments",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(environmentCmd)
}
