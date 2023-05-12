package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a lifecycle job",
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

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, lifecycle.Id, utils.JobType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Lifecycle %s stopped!", pterm.FgBlue.Sprintf(lifecycleName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping lifecycle %s in progress..", pterm.FgBlue.Sprintf(lifecycleName)))
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleStopCmd)
	lifecycleStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleStopCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Name")
	lifecycleStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")

	_ = lifecycleStopCmd.MarkFlagRequired("lifecycle")
}
