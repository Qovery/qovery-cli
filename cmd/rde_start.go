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

var rdeStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start (deploy) an RDE",
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

		_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(ctx(), child.EnvId).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("deploy failed: %w", err))
			os.Exit(1)
			panic("unreachable")
		}

		utils.Println(fmt.Sprintf("Request to start RDE %s has been queued..", pterm.FgBlue.Sprintf("%s", rdeName)))

		if watchFlag {
			time.Sleep(3 * time.Second)
			utils.WatchEnvironment(child.EnvId, qovery.STATEENUM_DEPLOYED, client)
		}
	},
}

func init() {
	rdeCmd.AddCommand(rdeStartCmd)
	rdeStartCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeStartCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeStartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch deployment status until it's ready or an error occurs")

	_ = rdeStartCmd.MarkFlagRequired("name")
}
