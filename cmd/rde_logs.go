package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rdeLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Fetch recent logs from an RDE workspace",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		child, err := rdeFindChildByName(client, orgId, fmt.Sprintf("rde-%s", rdeName))
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable")
		}

		// Find the first application in the environment
		apps, _, err := client.ApplicationsAPI.ListApplication(ctx(), child.EnvId).Execute()
		if err != nil || len(apps.GetResults()) == 0 {
			utils.PrintlnError(fmt.Errorf("no applications found in RDE %s", rdeName))
			os.Exit(1)
			panic("unreachable")
		}

		appId := apps.GetResults()[0].Id
		appName := apps.GetResults()[0].GetName()

		utils.Println(fmt.Sprintf("Fetching logs for RDE %s (service: %s)...", rdeName, appName))
		utils.Println("")

		logs, _, err := client.ApplicationLogsAPI.ListApplicationLog(ctx(), appId).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("failed to fetch logs: %w", err))
			os.Exit(1)
			panic("unreachable")
		}

		logResults := logs.GetResults()
		// Show last 50 lines
		start := 0
		if len(logResults) > 50 {
			start = len(logResults) - 50
		}

		for _, logEntry := range logResults[start:] {
			msg := logEntry.GetMessage()
			if msg != "" {
				utils.Println(msg)
			}
		}
	},
}

func init() {
	rdeCmd.AddCommand(rdeLogsCmd)
	rdeLogsCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeLogsCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeLogsCmd.MarkFlagRequired("name")
}
