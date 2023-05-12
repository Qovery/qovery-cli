package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var cronjobStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a cronjob",
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

		cronjobs, err := ListCronjobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs, cronjobName)

		if cronjob == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, cronjob.Id, utils.JobType, watchFlag)

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
			utils.Println(fmt.Sprintf("Cronjob %s stopped!", pterm.FgBlue.Sprintf(cronjobName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping cronjob %s in progress..", pterm.FgBlue.Sprintf(cronjobName)))
		}
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobStopCmd)
	cronjobStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobStopCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")

	_ = cronjobStopCmd.MarkFlagRequired("cronjob")
}
