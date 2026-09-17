package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
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

If --name is provided, upgrades a single RDE. Otherwise, upgrades all RDEs.
When upgrading multiple RDEs with reclone, environments are deleted in parallel
for faster execution.`,
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

			if rdeUpgradeStrategy == "image" {
				rdeUpgradeImage(client, child)
			} else {
				rdeUpgradeRecloneSingle(client, child)
			}
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

			if rdeUpgradeStrategy == "image" {
				for _, child := range children {
					rdeUpgradeImage(client, &child)
				}
			} else {
				rdeUpgradeRecloneAll(client, children)
			}
			utils.Println("\nAll RDEs upgraded.")
		}
	},
}

// rdeUpgradeImage triggers a redeploy of an RDE environment.
func rdeUpgradeImage(client *qovery.APIClient, child *rdeChildInfo) {
	name := strings.TrimPrefix(child.ProjectName, "rde-")
	utils.Println(fmt.Sprintf("  Upgrading %s (strategy: image - sync from blueprint and deploy)...", pterm.FgBlue.Sprintf("%s", name)))

	// Resolve blueprint environment ID
	bpEnvInfo, err := rdeFindBlueprintEnv(client, child.BlueprintProjectId)
	if err != nil || bpEnvInfo == nil {
		utils.Println(fmt.Sprintf("    ERROR: Could not find blueprint environment for %s", name))
		return
	}

	// Sync service configurations from blueprint
	synced := rdeSyncServicesFromBlueprint(client, bpEnvInfo.EnvId, child.EnvId)
	if synced == 0 {
		utils.Println(fmt.Sprintf("    No services matched between blueprint and %s, deploying as-is...", name))
	} else {
		utils.Println(fmt.Sprintf("    Synced %d service(s) from blueprint.", synced))
	}

	// Deploy
	_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(ctx(), child.EnvId).Execute()
	if err != nil {
		utils.Println(fmt.Sprintf("    WARNING: Deploy failed for %s: %v", name, err))
	} else {
		utils.Println(fmt.Sprintf("    Request to deploy %s has been queued..", pterm.FgBlue.Sprintf("%s", name)))
	}
}

// rdeUpgradeRecloneSingle upgrades a single RDE by deleting its environment, waiting, and re-cloning from the blueprint.
func rdeUpgradeRecloneSingle(client *qovery.APIClient, child *rdeChildInfo) {
	name := strings.TrimPrefix(child.ProjectName, "rde-")
	utils.Println(fmt.Sprintf("  Upgrading %s (strategy: reclone - full re-clone from blueprint)...", pterm.FgBlue.Sprintf("%s", name)))
	utils.Println("    WARNING: Uncommitted changes will be lost. Code in git is safe.")

	// Resolve blueprint environment ID
	bpEnvInfo, err := rdeFindBlueprintEnv(client, child.BlueprintProjectId)
	if err != nil || bpEnvInfo == nil {
		utils.Println(fmt.Sprintf("    ERROR: Could not find blueprint environment for %s", name))
		return
	}

	// Get blueprint cluster ID
	bpClusterId := rdeGetBlueprintClusterId(client, bpEnvInfo.EnvId)

	// Preserve owner email
	ownerEmail := child.OwnerEmail

	// Delete the current environment
	utils.Println(fmt.Sprintf("    Deleting environment %s...", pterm.FgBlue.Sprintf("%s", child.EnvName)))
	_, _ = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), child.EnvId).Execute()

	// Wait for deletion
	utils.Println("    Waiting for deletion to complete...")
	rdeWaitForEnvsDeletion(client, []string{child.EnvId}, 120*time.Second)

	// Re-clone from blueprint environment
	newEnv := rdeCloneFromBlueprint(client, child, bpEnvInfo.EnvId, bpClusterId)
	if newEnv == nil {
		return
	}

	// Restore owner email
	if ownerEmail != "" {
		_ = utils.CreateEnvironmentVariable(client, child.ProjectId, newEnv.Id, rdeOwnerEmailVar, ownerEmail, false)
	}

	// Update TTL job
	rdeUpdateTTLJob(client, newEnv.Id)

	// Deploy
	_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(ctx(), newEnv.Id).Execute()
	if err != nil {
		utils.Println(fmt.Sprintf("    WARNING: Deploy failed after re-clone for %s: %v", name, err))
	} else {
		utils.Println(fmt.Sprintf("    Re-cloned and deploying %s (env: %s)", pterm.FgBlue.Sprintf("%s", name), newEnv.Id))
	}
}

// rdeUpgradeRecloneAll upgrades multiple RDEs in parallel phases:
// Phase 1: Delete all environments (fire all delete requests)
// Phase 2: Wait for all deletions to complete
// Phase 3: Re-clone all from their blueprint
// Phase 4: Deploy all
func rdeUpgradeRecloneAll(client *qovery.APIClient, children []rdeChildInfo) {
	utils.Println("  WARNING: Uncommitted changes will be lost. Code in git is safe.")
	utils.Println("")

	// Pre-resolve all blueprint env IDs and cluster IDs (grouped by blueprint project ID)
	type blueprintRef struct {
		envId     string
		clusterId string
	}
	bpRefMap := make(map[string]*blueprintRef)
	for _, child := range children {
		if _, ok := bpRefMap[child.BlueprintProjectId]; !ok {
			bpEnvInfo, err := rdeFindBlueprintEnv(client, child.BlueprintProjectId)
			if err == nil && bpEnvInfo != nil {
				clusterId := rdeGetBlueprintClusterId(client, bpEnvInfo.EnvId)
				bpRefMap[child.BlueprintProjectId] = &blueprintRef{
					envId:     bpEnvInfo.EnvId,
					clusterId: clusterId,
				}
			}
		}
	}

	// Phase 1: Delete all environments
	utils.Println("  Phase 1/4: Deleting old environments...")
	var envIdsToWait []string
	for _, child := range children {
		name := strings.TrimPrefix(child.ProjectName, "rde-")
		_, _ = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), child.EnvId).Execute()
		envIdsToWait = append(envIdsToWait, child.EnvId)
		utils.Println(fmt.Sprintf("    Delete requested: %s", pterm.FgBlue.Sprintf("%s", name)))
	}

	// Phase 2: Wait for all deletions
	utils.Println("")
	utils.Println("  Phase 2/4: Waiting for deletions to complete...")
	rdeWaitForEnvsDeletion(client, envIdsToWait, 180*time.Second)
	utils.Println("    Deletions complete.")

	// Phase 3: Re-clone all from blueprint
	utils.Println("")
	utils.Println("  Phase 3/4: Cloning from blueprint...")
	type cloneResult struct {
		child  rdeChildInfo
		newEnv *qovery.Environment
	}
	var results []cloneResult
	for _, child := range children {
		name := strings.TrimPrefix(child.ProjectName, "rde-")
		ref, ok := bpRefMap[child.BlueprintProjectId]
		if !ok || ref == nil {
			utils.Println(fmt.Sprintf("    ERROR: No blueprint environment found for %s, skipping", name))
			continue
		}

		newEnv := rdeCloneFromBlueprint(client, &child, ref.envId, ref.clusterId)
		if newEnv == nil {
			continue
		}

		// Restore owner email
		if child.OwnerEmail != "" {
			_ = utils.CreateEnvironmentVariable(client, child.ProjectId, newEnv.Id, rdeOwnerEmailVar, child.OwnerEmail, false)
		}

		// Update TTL job
		rdeUpdateTTLJob(client, newEnv.Id)

		results = append(results, cloneResult{child: child, newEnv: newEnv})
		utils.Println(fmt.Sprintf("    Cloned: %s (env: %s)", pterm.FgBlue.Sprintf("%s", name), newEnv.Id))
	}

	// Phase 4: Deploy all
	utils.Println("")
	utils.Println("  Phase 4/4: Deploying...")
	for _, r := range results {
		name := strings.TrimPrefix(r.child.ProjectName, "rde-")
		_, _, err := client.EnvironmentActionsAPI.DeployEnvironment(ctx(), r.newEnv.Id).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Deploy failed for %s: %v", name, err))
		} else {
			utils.Println(fmt.Sprintf("    Request to deploy %s has been queued..", pterm.FgBlue.Sprintf("%s", name)))
		}
	}
}

// rdeCloneFromBlueprint clones the blueprint environment into an RDE's project.
func rdeCloneFromBlueprint(client *qovery.APIClient, child *rdeChildInfo, blueprintEnvId string, clusterId string) *qovery.Environment {
	name := strings.TrimPrefix(child.ProjectName, "rde-")

	cloneReq := qovery.CloneEnvironmentRequest{
		Name:      "workspace",
		ProjectId: &child.ProjectId,
		Mode:      qovery.ENVIRONMENTMODEENUM_DEVELOPMENT.Ptr(),
	}

	if clusterId != "" {
		cloneReq.ClusterId = &clusterId
	}

	newEnv, _, err := client.EnvironmentActionsAPI.CloneEnvironment(ctx(), blueprintEnvId).
		CloneEnvironmentRequest(cloneReq).Execute()
	if err != nil {
		utils.Println(fmt.Sprintf("    ERROR: Re-clone failed for %s: %v", name, err))
		return nil
	}

	return newEnv
}

func init() {
	rdeCmd.AddCommand(rdeUpgradeCmd)
	rdeUpgradeCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name (omit to upgrade all)")
	rdeUpgradeCmd.Flags().StringVarP(&rdeUpgradeStrategy, "strategy", "s", "image", "Upgrade strategy: 'image' (sync source and deploy) or 'reclone' (full re-clone)")
	rdeUpgradeCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name (when upgrading all)")
	rdeUpgradeCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	// Sync scope flags (used with --strategy image)
	rdeUpgradeCmd.Flags().BoolVarP(&rdeSyncAll, "sync-all", "", false, "Sync all config from blueprint (resources, ports, healthchecks, storage)")
	rdeUpgradeCmd.Flags().BoolVarP(&rdeSyncResources, "sync-resources", "", false, "Also sync CPU, memory, and instance counts from blueprint")
	rdeUpgradeCmd.Flags().BoolVarP(&rdeSyncPorts, "sync-ports", "", false, "Also sync port configuration from blueprint")
	rdeUpgradeCmd.Flags().BoolVarP(&rdeSyncHealthchecks, "sync-healthchecks", "", false, "Also sync health check configuration from blueprint")
	rdeUpgradeCmd.Flags().BoolVarP(&rdeSyncStorage, "sync-storage", "", false, "Also sync storage volumes from blueprint")
}
