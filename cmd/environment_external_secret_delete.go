package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentExternalSecretDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete environment external secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		client := utils.GetQoveryClient(tokenType, token)

		organizationId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		projects, _, err := client.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()
		checkError(err)

		project := utils.FindByProjectName(projects.GetResults(), projectName)
		if project == nil {
			utils.PrintlnError(fmt.Errorf("project %s not found", projectName))
			utils.PrintlnInfo("You can list all projects with: qovery project list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		checkError(err)

		environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)
		if environment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", environmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.DeleteEnvironmentVar(client, environment.Id, utils.Key)
		checkError(err)

		utils.Println(fmt.Sprintf("External secret %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentExternalSecretCmd.AddCommand(environmentExternalSecretDeleteCmd)
	environmentExternalSecretDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentExternalSecretDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentExternalSecretDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentExternalSecretDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")

	_ = environmentExternalSecretDeleteCmd.MarkFlagRequired("project")
	_ = environmentExternalSecretDeleteCmd.MarkFlagRequired("environment")
	_ = environmentExternalSecretDeleteCmd.MarkFlagRequired("key")
}
