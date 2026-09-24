package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var stageName string
var serviceName string
var newStageName string
var stageDescription string

var environmentStageCmd = &cobra.Command{
	Use:   "stage",
	Short: "Manage deployment stages",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentStageCmd)
}
