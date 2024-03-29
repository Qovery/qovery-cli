package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var helmName string
var helmNames string
var targetHelmName string
var chartVersion string
var chartName string
var chartGitCommitId string
var charGitCommitBranch string
var valuesOverrideCommitId string
var valuesOverrideCommitBranch string
var helmCustomDomain string

var helmCmd = &cobra.Command{
	Use:   "helm",
	Short: "Manage helms",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(helmCmd)
}
