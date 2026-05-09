package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeDeleteAllCmd = &cobra.Command{
	Use:   "delete-all",
	Short: "Delete ALL RDE environments",
	Long: `Delete all RDE environments permanently. Requires --confirm flag.

This will delete the environment, project, RBAC role, and API token for each RDE.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if !rdeConfirmFlag {
			utils.PrintlnError(fmt.Errorf("this will delete ALL RDE environments permanently"))
			utils.Println("Run with --confirm to proceed: qovery rde delete-all --confirm")
			os.Exit(1)
			panic("unreachable")
		}

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

		utils.Println(fmt.Sprintf("Deleting %d RDE environment(s)...", len(children)))
		utils.Println("")

		for _, child := range children {
			// Extract the RDE name from project name (strip "rde-" prefix if present)
			name := child.ProjectName
			if strings.HasPrefix(name, "rde-") {
				name = strings.TrimPrefix(name, "rde-")
			}

			utils.Println(fmt.Sprintf("=== Deleting: %s ===", child.ProjectName))

			// Stop environment
			if child.EnvId != "" {
				status, _ := rdeGetEnvStatus(client, child.EnvId)
				if status != qovery.STATEENUM_STOPPED && status != "" {
					utils.Println("  Stopping environment...")
					_, _, _ = client.EnvironmentActionsAPI.StopEnvironment(ctx(), child.EnvId).Execute()
					time.Sleep(2 * time.Second)
				}

				utils.Println("  Deleting environment...")
				_, _ = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), child.EnvId).Execute()
			}

			// Delete project
			utils.Println(fmt.Sprintf("  Deleting project %s...", child.ProjectName))
			_, _ = client.ProjectMainCallsAPI.DeleteProject(ctx(), child.ProjectId).Execute()

			// Cleanup role and token
			rdeCleanupRoleAndToken(client, orgId, name)

			utils.Println("")
		}

		utils.Println("All RDE environments deleted.")
	},
}

func init() {
	rdeCmd.AddCommand(rdeDeleteAllCmd)
	rdeDeleteAllCmd.Flags().BoolVarP(&rdeConfirmFlag, "confirm", "", false, "Confirm deletion of all RDE environments")
	rdeDeleteAllCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name")
	rdeDeleteAllCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
}
