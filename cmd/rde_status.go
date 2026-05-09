package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
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

		rows := [][]string{
			{"RDE", pterm.FgBlue.Sprintf("%s", child.ProjectName)},
			{"Project", child.ProjectId},
			{"Environment", fmt.Sprintf("%s (%s)", child.EnvName, child.EnvId)},
			{"Blueprint", fmt.Sprintf("%s (%s)", bpName, child.BlueprintProjectId)},
		}

		status, err := rdeGetEnvStatus(client, child.EnvId)
		if err == nil {
			rows = append(rows, []string{"Status", utils.GetStatusTextWithColor(status)})
		}

		if status == qovery.STATEENUM_DEPLOYED || status == qovery.STATEENUM_RESTARTED {
			url := rdeGetWorkspaceUrl(client, child.EnvId)
			if url != "" {
				rows = append(rows, []string{"Workspace", url})
			}
		}

		lastDeploy := rdeGetLastDeployTime(client, child.EnvId)
		uptime := rdeFormatUptime(lastDeploy)
		rows = append(rows, []string{"Uptime", uptime})
		rows = append(rows, []string{"Console", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, child.ProjectId, child.EnvId)})

		rdePrintKeyValueTable(rows)

		// List services
		utils.Println("")
		rdePrintEnvServices(client, child.EnvId)
	},
}

func init() {
	rdeCmd.AddCommand(rdeStatusCmd)
	rdeStatusCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeStatusCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeStatusCmd.MarkFlagRequired("name")
}
