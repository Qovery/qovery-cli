package cmd

import (
	"context"
	"fmt"
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
		checkError(err)

		client := utils.GetQoveryClient(tokenType, token)

		organizationId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		projects, _, err := client.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()
		checkError(err)

		project := utils.FindByProjectName(projects.GetResults(), projectName)

		err = utils.DeleteProjectVar(client, project.Id, utils.Key)
		checkError(err)

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
