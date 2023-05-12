package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var lifecycleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a lifecycle job",
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

		msg, err := utils.DeleteService(client, envId, lifecycle.Id, utils.JobType, watchFlag)

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
			utils.Println(fmt.Sprintf("Lifecycle %s deleted!", pterm.FgBlue.Sprintf(lifecycleName)))
		} else {
			utils.Println(fmt.Sprintf("Deleting lifecycle %s in progress..", pterm.FgBlue.Sprintf(lifecycleName)))
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeleteCmd)
	lifecycleDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeleteCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle job status until it's ready or an error occurs")

	_ = lifecycleDeleteCmd.MarkFlagRequired("lifecycle")
}
