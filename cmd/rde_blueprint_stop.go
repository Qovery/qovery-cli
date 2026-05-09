package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeBlueprintStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a blueprint environment",
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

		if bp.EnvId == "" {
			utils.PrintlnError(fmt.Errorf("blueprint %s has no environment with %s set", bp.ProjectName, rdeBlueprintKeyVar))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.EnvironmentActionsAPI.StopEnvironment(context.Background(), bp.EnvId).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("stop failed: %w", err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Request to stop blueprint %s has been queued..", pterm.FgBlue.Sprintf("%s", bp.ProjectName)))

		if watchFlag {
			time.Sleep(3 * time.Second)
			utils.WatchEnvironment(bp.EnvId, qovery.STATEENUM_STOPPED, client)
		}
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintStopCmd)
	rdeBlueprintStopCmd.Flags().StringVarP(&rdeBlueprintProjectName, "project", "p", "", "Blueprint Project Name")
	rdeBlueprintStopCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeBlueprintStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch stop status until it completes or an error occurs")

	_ = rdeBlueprintStopCmd.MarkFlagRequired("project")
}
