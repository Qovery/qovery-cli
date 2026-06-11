package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create environment external secret",
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

		err = utils.CreateServiceExternalSecret(client, project.Id, environment.Id, "", utils.EnvironmentScope, utils.Key, utils.Reference, utils.SecretManagerAccessId, utils.MountPath)
		checkError(err)

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentExternalSecretCmd.AddCommand(environmentExternalSecretCreateCmd)
	environmentExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessId, "secret-manager-access-id", "", "", "Secret manager access ID")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&utils.EnvironmentScope, "scope", "", "ENVIRONMENT", "Scope of this external secret <PROJECT|ENVIRONMENT>")
	environmentExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = environmentExternalSecretCreateCmd.MarkFlagRequired("project")
	_ = environmentExternalSecretCreateCmd.MarkFlagRequired("environment")
	_ = environmentExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = environmentExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = environmentExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-id")
}
