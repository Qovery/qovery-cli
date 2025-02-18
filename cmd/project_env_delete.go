package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete project variable or secret",
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

		err = utils.DeleteProjectVar(client, project.Id, utils.Key)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Variable %s has been deleted", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	projectEnvCmd.AddCommand(projectEnvDeleteCmd)
	projectEnvDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectEnvDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	projectEnvDeleteCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Project variable or secret key")

	_ = projectEnvDeleteCmd.MarkFlagRequired("project")
	_ = projectEnvDeleteCmd.MarkFlagRequired("key")
}
