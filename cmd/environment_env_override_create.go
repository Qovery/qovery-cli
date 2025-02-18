package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvOverrideCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Override environment variable or secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		organizationId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		projects, _, err := client.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		project := utils.FindByProjectName(projects.GetResults(), projectName)

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)

		if environment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", environmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateEnvironmentOverride(client, projectId, environment.Id, utils.Key, utils.Value, utils.EnvironmentScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("%s has been overidden", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentEnvOverrideCmd.AddCommand(environmentEnvOverrideCreateCmd)
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&utils.Value, "value", "", "", "Environment variable or secret value")
	environmentEnvOverrideCreateCmd.Flags().StringVarP(&utils.EnvironmentScope, "scope", "", "ENVIRONMENT", "Scope of this alias <PROJECT|ENVIRONMENT>")

	_ = environmentEnvOverrideCreateCmd.MarkFlagRequired("project")
	_ = environmentEnvOverrideCreateCmd.MarkFlagRequired("environment")
	_ = environmentEnvOverrideCreateCmd.MarkFlagRequired("key")
	_ = environmentEnvOverrideCreateCmd.MarkFlagRequired("value")
}
