package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rdeStopAllCmd = &cobra.Command{
	Use:   "stop-all",
	Short: "Stop all RDE environments",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		var children []rdeChildInfo

		if rdeBlueprintProjectName != "" {
			bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable")
			}
			children, err = rdeListChildren(client, orgId, bp.ProjectId)
			checkError(err)
		} else {
			children, err = rdeListAllChildren(client, orgId)
			checkError(err)
		}

		if len(children) == 0 {
			utils.Println("No RDE instances found.")
			return
		}

		utils.Println(fmt.Sprintf("Stopping %d RDE environment(s)...", len(children)))
		for _, child := range children {
			if child.EnvId != "" {
				_, _, err := client.EnvironmentActionsAPI.StopEnvironment(ctx(), child.EnvId).Execute()
				if err != nil {
					utils.Println(fmt.Sprintf("  Failed to stop: %s (%v)", pterm.FgBlue.Sprintf("%s", child.ProjectName), err))
				} else {
					utils.Println(fmt.Sprintf("  Request to stop %s has been queued..", pterm.FgBlue.Sprintf("%s", child.ProjectName)))
				}
			}
		}
		utils.Println("Done.")
	},
}

func init() {
	rdeCmd.AddCommand(rdeStopAllCmd)
	rdeStopAllCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name")
	rdeStopAllCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
}
