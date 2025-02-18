package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete environment variable or secret",
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

		err = utils.DeleteEnvironmentVar(client, environment.Id, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	environmentEnvCmd.AddCommand(environmentEnvDeleteCmd)
	environmentEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")

	_ = environmentEnvDeleteCmd.MarkFlagRequired("project")
	_ = environmentEnvDeleteCmd.MarkFlagRequired("environment")
	_ = environmentEnvDeleteCmd.MarkFlagRequired("key")
}
