package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed status of an RDE",
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

		bpName := rdeBlueprintNameForProjectId(client, child.BlueprintProjectId)

		utils.Println(fmt.Sprintf("RDE: %s", child.ProjectName))
		utils.Println(fmt.Sprintf("  Project:     %s", child.ProjectId))
		utils.Println(fmt.Sprintf("  Environment: %s (%s)", child.EnvName, child.EnvId))
		utils.Println(fmt.Sprintf("  Blueprint:   %s (%s)", bpName, child.BlueprintProjectId))

		status, err := rdeGetEnvStatus(client, child.EnvId)
		if err == nil {
			utils.Println(fmt.Sprintf("  Status:      %s", utils.GetStatusTextWithColor(status)))
		}

		if status == qovery.STATEENUM_DEPLOYED || status == qovery.STATEENUM_RESTARTED {
			url := rdeGetWorkspaceUrl(client, child.EnvId)
			if url != "" {
				utils.Println(fmt.Sprintf("  Workspace:   %s", url))
			}
		}

		lastDeploy := rdeGetLastDeployTime(client, child.EnvId)
		uptime := rdeFormatUptime(lastDeploy)
		utils.Println(fmt.Sprintf("  Uptime:      %s", uptime))
		utils.Println(fmt.Sprintf("  Console:     https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, child.ProjectId, child.EnvId))

		// List services
		utils.Println("")
		utils.Println("  Services:")
		rdePrintEnvServices(client, child.EnvId)
	},
}

func init() {
	rdeCmd.AddCommand(rdeStatusCmd)
	rdeStatusCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeStatusCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeStatusCmd.MarkFlagRequired("name")
}
