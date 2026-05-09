package cmd

import (
	"fmt"
	"os"

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

		utils.Println(fmt.Sprintf("Blueprint: %s", bp.ProjectName))
		utils.Println(fmt.Sprintf("  Project:     %s", bp.ProjectId))

		if bp.EnvId == "" {
			utils.Println("  Environment: (none)")
			return
		}

		utils.Println(fmt.Sprintf("  Environment: %s (%s)", bp.EnvName, bp.EnvId))

		status, err := rdeGetEnvStatus(client, bp.EnvId)
		if err == nil {
			utils.Println(fmt.Sprintf("  Status:      %s", utils.GetStatusTextWithColor(status)))
		}

		utils.Println(fmt.Sprintf("  Console:     https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, bp.ProjectId, bp.EnvId))

		// Count children
		children, err := rdeListChildren(client, orgId, bp.ProjectId)
		if err == nil {
			utils.Println(fmt.Sprintf("  Children:    %d RDE(s)", len(children)))
		}

		// List services
		utils.Println("")
		utils.Println("  Services:")

		rdePrintEnvServices(client, bp.EnvId)
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintStatusCmd)
	rdeBlueprintStatusCmd.Flags().StringVarP(&rdeBlueprintProjectName, "project", "p", "", "Blueprint Project Name")
	rdeBlueprintStatusCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeBlueprintStatusCmd.MarkFlagRequired("project")
}
