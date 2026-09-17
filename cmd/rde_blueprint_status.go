package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rdeBlueprintStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed status of a blueprint",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		rows := [][]string{
			{"Blueprint", pterm.FgBlue.Sprintf("%s", bp.ProjectName)},
			{"Project", bp.ProjectId},
		}

		if bp.EnvId == "" {
			rows = append(rows, []string{"Environment", "(none)"})
			rdePrintKeyValueTable(rows)
			return
		}

		rows = append(rows, []string{"Environment", fmt.Sprintf("%s (%s)", bp.EnvName, bp.EnvId)})

		status, err := rdeGetEnvStatus(client, bp.EnvId)
		if err == nil {
			rows = append(rows, []string{"Status", utils.GetStatusTextWithColor(status)})
		}

		rows = append(rows, []string{"Console", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, bp.ProjectId, bp.EnvId)})

		// Count children
		children, err := rdeListChildren(client, orgId, bp.ProjectId)
		if err == nil {
			rows = append(rows, []string{"Children", fmt.Sprintf("%d RDE(s)", len(children))})
		}

		rdePrintKeyValueTable(rows)

		// List services
		utils.Println("")
		rdePrintEnvServices(client, bp.EnvId)
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintStatusCmd)
	rdeBlueprintStatusCmd.Flags().StringVarP(&rdeBlueprintProjectName, "project", "p", "", "Blueprint Project Name")
	rdeBlueprintStatusCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeBlueprintStatusCmd.MarkFlagRequired("project")
}
