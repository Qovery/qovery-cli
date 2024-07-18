package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var (
	adminPublishEnvironmentDeploymentRulesCmd = &cobra.Command{
		Use:   "publish-environment-deployment-rules",
		Short: "Republish environment deployment rules to scheduler",
		Run: func(cmd *cobra.Command, args []string) {
			publishEnvironmentDeploymentRules()
		},
	}
)

func init() {
	adminCmd.AddCommand(adminPublishEnvironmentDeploymentRulesCmd)
}

func publishEnvironmentDeploymentRules() {
	err := pkg.PublishEnvironmentDeploymentRules()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
}
