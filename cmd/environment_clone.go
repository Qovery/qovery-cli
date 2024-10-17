package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/pkg/environment"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var environmentCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		orgId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var newEnvId = environment.EnvironmentClone(client, organizationName, projectName, environmentName, newEnvironmentName, clusterName, environmentType, applyDeploymentRule, orgId, envId)

		utils.Println("your new environment ID is: " + newEnvId.Id)
	},
}

func init() {
	environmentCmd.AddCommand(environmentCloneCmd)
	environmentCloneCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	environmentCloneCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project Name")
	environmentCloneCmd.Flags().StringVarP(&environmentName, "environment", "e", "", "Environment Name to clone")
	environmentCloneCmd.Flags().StringVarP(&newEnvironmentName, "new-environment-name", "n", "", "New Environment Name")
	environmentCloneCmd.Flags().StringVarP(&clusterName, "cluster", "c", "", "Cluster Name where to clone the environment")
	environmentCloneCmd.Flags().StringVarP(&environmentType, "environment-type", "t", "", "Environment type for the new environment (DEVELOPMENT|STAGING|PRODUCTION)")
	environmentCloneCmd.Flags().BoolVarP(&applyDeploymentRule, "apply-deployment-rule", "", false, "Enable applying deployment rules on the new environment instead of having a pristine clone. Default: false")

	_ = environmentCloneCmd.MarkFlagRequired("new-environment-name")
}
