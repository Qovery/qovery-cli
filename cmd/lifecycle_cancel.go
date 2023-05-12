package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var lifecycleCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a lifecycle deployment",
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

		msg, err := utils.CancelServiceDeployment(client, envId, lifecycle.Id, utils.JobType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		utils.Println(fmt.Sprintf("Lifecycle %s deployment cancelled!", pterm.FgBlue.Sprintf(lifecycleName)))
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleCancelCmd)
	lifecycleCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleCancelCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cancel until it's done or an error occurs")

	_ = lifecycleCancelCmd.MarkFlagRequired("lifecycle")
}
