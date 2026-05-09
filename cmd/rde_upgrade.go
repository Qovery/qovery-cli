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

var rdeUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade RDE(s) from the updated blueprint",
	Long: `Upgrade one or all RDE environments using one of two strategies:

  image   (default) - Redeploy the environment (re-pulls latest images)
  reclone           - Delete the environment, re-clone from blueprint, and deploy
                      WARNING: uncommitted changes will be lost with reclone

If --name is provided, upgrades a single RDE. Otherwise, upgrades all RDEs.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if rdeUpgradeStrategy == "" {
			rdeUpgradeStrategy = "image"
		}
		if rdeUpgradeStrategy != "image" && rdeUpgradeStrategy != "reclone" {
			utils.PrintlnError(fmt.Errorf("unknown strategy '%s'. Use 'image' or 'reclone'", rdeUpgradeStrategy))
			os.Exit(1)
			panic("unreachable")
		}

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		if rdeName != "" {
			// Upgrade a single RDE
			child, err := rdeFindChildByName(client, orgId, fmt.Sprintf("rde-%s", rdeName))
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable")
			}
			rdeUpgradeOne(client, orgId, child)
		} else {
			// Upgrade all RDEs
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

			utils.Println(fmt.Sprintf("Upgrading %d RDE(s) (strategy: %s)...", len(children), rdeUpgradeStrategy))
			for _, child := range children {
				rdeUpgradeOne(client, orgId, &child)
			}
			utils.Println("\nAll RDEs upgraded.")
		}
	},
}

func rdeUpgradeOne(client *qovery.APIClient, orgId string, child *rdeChildInfo) {
	name := strings.TrimPrefix(child.ProjectName, "rde-")

	if rdeUpgradeStrategy == "image" {
		utils.Println(fmt.Sprintf("  Upgrading %s (strategy: image - redeploy only)...", name))
		_, _, err := client.EnvironmentActionsAPI.DeployEnvironment(ctx(), child.EnvId).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("  WARNING: Deploy failed for %s: %v", name, err))
		} else {
			utils.Println(fmt.Sprintf("  Deploy triggered for %s.", name))
		}
	} else {
		// reclone strategy
		utils.Println(fmt.Sprintf("  Upgrading %s (strategy: reclone - full re-clone from blueprint)...", name))
		utils.Println("    WARNING: Uncommitted changes will be lost. Code in git is safe.")

		// Stop and delete the current environment
		_, _, _ = client.EnvironmentActionsAPI.StopEnvironment(ctx(), child.EnvId).Execute()
		time.Sleep(2 * time.Second)
		_, _ = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), child.EnvId).Execute()
		time.Sleep(2 * time.Second)

		// Re-clone from blueprint
		cloneReq := qovery.CloneEnvironmentRequest{
			Name:      "workspace",
			ProjectId: &child.ProjectId,
			Mode:      qovery.ENVIRONMENTMODEENUM_DEVELOPMENT.Ptr(),
		}

		newEnv, _, err := client.EnvironmentActionsAPI.CloneEnvironment(ctx(), child.BlueprintProjectId).
			CloneEnvironmentRequest(cloneReq).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    ERROR: Re-clone failed for %s: %v", name, err))
			return
		}

		// Wait a moment for the clone to be processed, but we need the blueprint env ID, not project ID
		// Find the blueprint environment
		bpEnvInfo, err := rdeFindBlueprintEnv(client, child.BlueprintProjectId)
		if err != nil || bpEnvInfo == nil {
			utils.Println(fmt.Sprintf("    ERROR: Could not find blueprint environment for %s", name))
			return
		}

		// Re-attempt clone from the actual blueprint environment
		_, _ = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), newEnv.Id).Execute()
		time.Sleep(1 * time.Second)

		newEnv, _, err = client.EnvironmentActionsAPI.CloneEnvironment(ctx(), bpEnvInfo.EnvId).
			CloneEnvironmentRequest(cloneReq).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    ERROR: Re-clone failed for %s: %v", name, err))
			return
		}

		// Update TTL job
		rdeUpdateTTLJob(client, newEnv.Id)

		// Deploy
		_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(ctx(), newEnv.Id).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Deploy failed after re-clone for %s: %v", name, err))
		} else {
			utils.Println(fmt.Sprintf("    Re-cloned and deploying: %s", newEnv.Id))
		}
	}
}

func init() {
	rdeCmd.AddCommand(rdeUpgradeCmd)
	rdeUpgradeCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name (omit to upgrade all)")
	rdeUpgradeCmd.Flags().StringVarP(&rdeUpgradeStrategy, "strategy", "s", "image", "Upgrade strategy: 'image' (redeploy) or 'reclone' (full re-clone)")
	rdeUpgradeCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name (when upgrading all)")
	rdeUpgradeCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
}
