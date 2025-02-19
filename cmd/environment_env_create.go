package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create environment variable or secret",
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

		err = utils.CreateEnvironmentVariable(client, projectId, environment.Id, utils.Key, utils.Value, utils.IsSecret)
		checkError(err)

		utils.Println(fmt.Sprintf("Environment variable %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentEnvCmd.AddCommand(environmentEnvCreateCmd)
	environmentEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	environmentEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	environmentEnvCreateCmd.Flags().StringVarP(&utils.EnvironmentScope, "scope", "", "ENVIRONMENT", "Scope of this env var <PROJECT|ENVIRONMENT>")
	environmentEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This environment variable is a secret")

	_ = environmentEnvCreateCmd.MarkFlagRequired("project")
	_ = environmentEnvCreateCmd.MarkFlagRequired("environment")
	_ = environmentEnvCreateCmd.MarkFlagRequired("key")
	_ = environmentEnvCreateCmd.MarkFlagRequired("value")
}
