package cmd

import (
	"context"
	"fmt"
	"os"

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

		err = utils.CreateProjectVariable(client, project.Id, utils.Key, utils.Value, utils.IsSecret)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

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
