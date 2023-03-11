package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List application environment variables",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("envVar %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery envVar list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVars, _, err := client.ApplicationEnvironmentVariableApi.ListApplicationEnvironmentVariable(
			context.Background(),
			application.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		secrets, _, err := client.ApplicationSecretApi.ListApplicationSecrets(
			context.Background(),
			application.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVarLines := utils.NewEnvVarLines()

		for _, envVar := range envVars.GetResults() {
			envVarLines.Add(utils.FromEnvironmentVariableToEnvVarLineOutput(envVar))
		}

		for _, secret := range secrets.GetResults() {
			envVarLines.Add(utils.FromSecretToEnvVarLineOutput(secret))
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
	applicationEnvCmd.AddCommand(applicationEnvListCmd)
	applicationEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationEnvListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationEnvListCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	applicationEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")

	_ = applicationEnvListCmd.MarkFlagRequired("application")
}
