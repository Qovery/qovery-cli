package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvAliasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create environment variable or secret alias",
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

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		checkError(err)

		environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)

		if environment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", environmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateEnvironmentAlias(client, projectId, environment.Id, utils.Key, utils.Alias, utils.EnvironmentScope)
		checkError(err)

		utils.Println(fmt.Sprintf("Alias %s has been created", pterm.FgBlue.Sprintf("%s", utils.Alias)))
	},
}

func init() {
	environmentEnvAliasCmd.AddCommand(environmentEnvAliasCreateCmd)
	environmentEnvAliasCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvAliasCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvAliasCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvAliasCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	environmentEnvAliasCreateCmd.Flags().StringVarP(&utils.Alias, "alias", "", "", "Environment variable or secret alias")
	environmentEnvAliasCreateCmd.Flags().StringVarP(&utils.EnvironmentScope, "scope", "", "ENVIRONMENT", "Scope of this alias <PROJECT|ENVIRONMENT>")

	_ = environmentEnvAliasCreateCmd.MarkFlagRequired("project")
	_ = environmentEnvAliasCreateCmd.MarkFlagRequired("environment")
	_ = environmentEnvAliasCreateCmd.MarkFlagRequired("key")
	_ = environmentEnvAliasCreateCmd.MarkFlagRequired("alias")
}
