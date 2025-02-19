package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create project variable or secret",
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

		err = utils.CreateProjectVariable(client, project.Id, utils.Key, utils.Value, utils.IsSecret)
		checkError(err)

		utils.Println(fmt.Sprintf("Project variable %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	projectEnvCmd.AddCommand(projectEnvCreateCmd)
	projectEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	projectEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Project variable or secret key")
	projectEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Project variable or secret value")
	projectEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This Project variable is a secret")

	_ = projectEnvCreateCmd.MarkFlagRequired("project")
	_ = projectEnvCreateCmd.MarkFlagRequired("key")
	_ = projectEnvCreateCmd.MarkFlagRequired("value")
}
