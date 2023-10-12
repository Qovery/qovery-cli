package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var cronjobCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a cronjob deployment",
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

		msg, err := utils.CancelServiceDeployment(client, envId, cronjob.Id, utils.JobType, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		utils.Println(fmt.Sprintf("Cronjob %s deployment cancelled!", pterm.FgBlue.Sprintf(cronjobName)))
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobCancelCmd)
	cronjobCancelCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobCancelCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobCancelCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobCancelCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobCancelCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cancel until it's done or an error occurs")

	_ = cronjobCancelCmd.MarkFlagRequired("cronjob")
}
