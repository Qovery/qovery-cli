package cmd

import (
	"context"
	"fmt"
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
		checkError(err)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		projects, _, err := client.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()
		checkError(err)

		project := utils.FindByProjectName(projects.GetResults(), projectName)
		err = utils.UpdateProjectVariable(client, project.Id, utils.Key, utils.Value)
		checkError(err)

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
