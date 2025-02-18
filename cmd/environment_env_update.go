package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update environment variable or secret",
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

		err = utils.UpdateEnvironmentVariable(client, environment.Id, utils.Key, utils.Value)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentEnvCmd.AddCommand(environmentEnvUpdateCmd)
	environmentEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	environmentEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")

	_ = environmentEnvUpdateCmd.MarkFlagRequired("project")
	_ = environmentEnvUpdateCmd.MarkFlagRequired("environment")
	_ = environmentEnvUpdateCmd.MarkFlagRequired("key")
	_ = environmentEnvUpdateCmd.MarkFlagRequired("value")
}
