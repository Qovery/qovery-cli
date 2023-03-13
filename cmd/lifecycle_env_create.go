package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleEnvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create lifecycle environment variable or secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, projectId, envId, err := getContextResourcesId(client)

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
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycles with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if utils.IsSecret {
			err = utils.CreateSecret(client, projectId, envId, lifecycle.Id, utils.Key, utils.Value, utils.JobScope)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			utils.Println(fmt.Sprintf("Secret %s has been created", pterm.FgBlue.Sprintf(utils.Key)))
			return
		}

		err = utils.CreateEnvironmentVariable(client, projectId, envId, lifecycle.Id, utils.Key, utils.Value, utils.JobScope)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Environment variable %s has been created", pterm.FgBlue.Sprintf(utils.Key)))
	},
}

func init() {
	lifecycleEnvCmd.AddCommand(lifecycleEnvCreateCmd)
	lifecycleEnvCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleEnvCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleEnvCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleEnvCreateCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleEnvCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "Environment variable or secret key")
	lifecycleEnvCreateCmd.Flags().StringVarP(&utils.Value, "value", "v", "", "Environment variable or secret value")
	lifecycleEnvCreateCmd.Flags().StringVarP(&utils.JobScope, "scope", "", "JOB", "Scope of this env var <PROJECT|ENVIRONMENT|JOB>")
	lifecycleEnvCreateCmd.Flags().BoolVarP(&utils.IsSecret, "secret", "", false, "This environment variable is a secret")

	_ = lifecycleEnvCreateCmd.MarkFlagRequired("key")
	_ = lifecycleEnvCreateCmd.MarkFlagRequired("value")
	_ = lifecycleEnvCreateCmd.MarkFlagRequired("lifecycle")
}
