package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var projectEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List project variables",
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
		envVars, err := utils.ListProjectVariables(client, project.Id)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVarLines := utils.NewEnvVarLines()
		var variables []utils.EnvVarLineOutput

		for _, envVar := range envVars {
			s := utils.FromEnvironmentVariableToEnvVarLineOutput(envVar)
			variables = append(variables, s)
			envVarLines.Add(s)
		}

		if jsonFlag {
			utils.Println(utils.GetEnvVarJsonOutput(variables))
			return
		}

		err = utils.PrintTable(envVarLines.Header(utils.PrettyPrint), envVarLines.Lines(utils.ShowValues, utils.PrettyPrint))

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	projectEnvCmd.AddCommand(projectEnvListCmd)
	projectEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	projectEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	projectEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")
	projectEnvListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = projectEnvListCmd.MarkFlagRequired("project")
}
