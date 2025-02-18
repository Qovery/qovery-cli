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

		envVars, err := utils.ListEnvironmentVariables(client, environment.Id)

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
