package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cronjob environment variables",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs.GetResults(), cronjobName)

		if cronjob == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVars, _, err := client.JobEnvironmentVariableAPI.ListJobEnvironmentVariable(
			context.Background(),
			cronjob.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		secrets, _, err := client.JobSecretAPI.ListJobSecrets(
			context.Background(),
			cronjob.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVarLines := utils.NewEnvVarLines()
		var variables []utils.EnvVarLineOutput

		for _, envVar := range envVars.GetResults() {
			s := utils.FromEnvironmentVariableToEnvVarLineOutput(envVar)
			variables = append(variables, s)
			envVarLines.Add(s)
		}

		for _, secret := range secrets.GetResults() {
			s := utils.FromSecretToEnvVarLineOutput(secret)
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
	cronjobEnvCmd.AddCommand(cronjobEnvListCmd)
	cronjobEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobEnvListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobEnvListCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	cronjobEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")
	cronjobEnvListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = cronjobEnvListCmd.MarkFlagRequired("cronjob")
}
