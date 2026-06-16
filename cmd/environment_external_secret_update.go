package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentExternalSecretUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update environment external secret",
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

		err = utils.UpdateEnvironmentExternalSecret(client, environment.Id, utils.Key, utils.Reference, utils.SecretManagerAccessId)
		checkError(err)

		utils.Println(fmt.Sprintf("External secret %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentExternalSecretCmd.AddCommand(environmentExternalSecretUpdateCmd)
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "New reference to the secret in the secrets provider")
	environmentExternalSecretUpdateCmd.Flags().StringVarP(&utils.SecretManagerAccessId, "secret-manager-access-id", "", "", "New secret manager access ID")

	_ = environmentExternalSecretUpdateCmd.MarkFlagRequired("project")
	_ = environmentExternalSecretUpdateCmd.MarkFlagRequired("environment")
	_ = environmentExternalSecretUpdateCmd.MarkFlagRequired("key")
}
