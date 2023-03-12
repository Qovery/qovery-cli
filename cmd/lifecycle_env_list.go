package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List lifecycle environment variables",
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

		lifecycles, _, err := client.JobsApi.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles.GetResults(), lifecycleName)

		if lifecycle == nil {
			utils.PrintlnError(fmt.Errorf("envVar %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery envVar list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVars, _, err := client.JobEnvironmentVariableApi.ListJobEnvironmentVariable(
			context.Background(),
			lifecycle.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		secrets, _, err := client.JobSecretApi.ListJobSecrets(
			context.Background(),
			lifecycle.Id,
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
	lifecycleEnvCmd.AddCommand(lifecycleEnvListCmd)
	lifecycleEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleEnvListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleEnvListCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	lifecycleEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")

	_ = lifecycleEnvListCmd.MarkFlagRequired("lifecycle")
}
