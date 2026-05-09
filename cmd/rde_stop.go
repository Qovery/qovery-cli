package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop an RDE",
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

		_, _, err = client.EnvironmentActionsAPI.StopEnvironment(ctx(), child.EnvId).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("stop failed: %w", err))
			os.Exit(1)
			panic("unreachable")
		}

		utils.Println(fmt.Sprintf("Request to stop RDE %s has been queued..", pterm.FgBlue.Sprintf("%s", rdeName)))

		if watchFlag {
			time.Sleep(3 * time.Second)
			utils.WatchEnvironment(child.EnvId, qovery.STATEENUM_STOPPED, client)
		}
	},
}

func init() {
	rdeCmd.AddCommand(rdeStopCmd)
	rdeStopCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeStopCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch stop status until it completes or an error occurs")

	_ = rdeStopCmd.MarkFlagRequired("name")
}
