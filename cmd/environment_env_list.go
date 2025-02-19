package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var environmentEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variables",
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

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		checkError(err)

		environment := utils.FindByEnvironmentName(environments.GetResults(), environmentName)

		if environment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", environmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVars, err := utils.ListEnvironmentVariables(client, environment.Id)
		checkError(err)

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
		checkError(err)
	},
}

func init() {
	environmentEnvCmd.AddCommand(environmentEnvListCmd)
	environmentEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentEnvListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	environmentEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")
	environmentEnvListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = environmentEnvListCmd.MarkFlagRequired("project")
	_ = environmentEnvListCmd.MarkFlagRequired("environment")
}
