package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/pkg/environment"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var skipPausedServicesFlag bool

var environmentDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environment.EnvironmentDeploy(client, organizationName, projectName, environmentName, newEnvironmentName, clusterName, environmentType, applyDeploymentRule, envId, servicesJson, applicationNames, containerNames, lifecycleNames, cronjobNames, helmNames, skipPausedServicesFlag, watchFlag)
	},
}

func init() {
	environmentCmd.AddCommand(environmentDeployCmd)
	environmentDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeployCmd.Flags().StringVarP(&servicesJson, "services", "", "", "Services to deploy (JSON Format: https://api-doc.qovery.com/#tag/Environment-Actions/operation/deployAllServices)")
	environmentDeployCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Applications to deploy E.g. --applications app1:commit_id,app2:commit_id). If you omit the commit id, the same commit will be used")
	environmentDeployCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Containers to deploy E.g. --containers container1:image_tag,container2:image_tag). If you omit the image tag, the same image tag will be used")
	environmentDeployCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle to deploy E.g. --lifecycles job1:image_tag|git_commit_id,job2:image_tag|git_commit_id). If you omit the git commit id or image tag, the same version will be used")
	environmentDeployCmd.Flags().StringVarP(&cronjobNames, "cronjobs", "", "", "Cronjobs to deploy E.g. --cronjobs cronjob1:git_commit_id,cronjob2:git_commit_id). If you omit the git commit id, the same version will be used")
	environmentDeployCmd.Flags().StringVarP(&helmNames, "helms", "", "", "Helms to deploy E.g. --helms helm1:chart_version|git_commit_id,helm2:chart_version|git_commit_id). If you omit the chart version or git commit id, the same version will be used")
	environmentDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch environment status until it's ready or an error occurs")
	environmentDeployCmd.Flags().BoolVarP(&skipPausedServicesFlag, "skip-paused-services", "", false, "Skip paused services: paused services won't be started / deployed")
}
