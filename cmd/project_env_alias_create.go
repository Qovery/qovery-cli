package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvAliasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create project variable or secret alias",
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

		err = utils.CreateProjectAlias(client, project.Id, utils.Key, utils.Alias)
		checkError(err)

		utils.Println(fmt.Sprintf("Alias %s has been created", pterm.FgBlue.Sprintf("%s", utils.Alias)))
	},
}

func init() {
	projectEnvAliasCmd.AddCommand(projectEnvAliasCreateCmd)
	projectEnvAliasCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectEnvAliasCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	projectEnvAliasCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Project variable or secret key")
	projectEnvAliasCreateCmd.Flags().StringVarP(&utils.Alias, "alias", "", "", "Project variable or secret alias")

	_ = projectEnvAliasCreateCmd.MarkFlagRequired("project")
	_ = projectEnvAliasCreateCmd.MarkFlagRequired("key")
	_ = projectEnvAliasCreateCmd.MarkFlagRequired("alias")
}
