package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update project variable or secret",
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
		err = utils.UpdateProjectVariable(client, project.Id, utils.Key, utils.Value)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Project variable %s has been updated", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	projectEnvCmd.AddCommand(projectEnvUpdateCmd)
	projectEnvUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectEnvUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	projectEnvUpdateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Project variable or secret key")
	projectEnvUpdateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Project variable or secret value")

	_ = projectEnvUpdateCmd.MarkFlagRequired("project")
	_ = projectEnvUpdateCmd.MarkFlagRequired("key")
	_ = projectEnvUpdateCmd.MarkFlagRequired("value")
}
