package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all RDE instances",
	Long: `List all Remote Development Environments, optionally filtered by blueprint.

Shows name, blueprint, status, uptime, and workspace URL for each RDE.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		var children []rdeChildInfo

		if rdeBlueprintProjectName != "" {
			// Filter by specific blueprint
			bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable")
			}
			children, err = rdeListChildren(client, orgId, bp.ProjectId)
			checkError(err)
		} else {
			// List all children across all blueprints
			children, err = rdeListAllChildren(client, orgId)
			checkError(err)
		}

		if len(children) == 0 {
			utils.Println("No RDE instances found.")
			return
		}

		if jsonFlag {
			var results []interface{}
			for _, child := range children {
				status := ""
				url := ""
				if child.EnvId != "" {
					s, err := rdeGetEnvStatus(client, child.EnvId)
					if err == nil {
						status = string(s)
					}
					if s == qovery.STATEENUM_DEPLOYED {
						url = rdeGetWorkspaceUrl(client, child.EnvId)
					}
				}
				bpName := rdeBlueprintNameForProjectId(client, child.BlueprintProjectId)
				results = append(results, map[string]interface{}{
					"project_id":     child.ProjectId,
					"project_name":   child.ProjectName,
					"env_id":         child.EnvId,
					"env_name":       child.EnvName,
					"blueprint_id":   child.BlueprintProjectId,
					"blueprint_name": bpName,
					"status":         status,
					"workspace_url":  url,
				})
			}
			j, _ := json.Marshal(results)
			utils.Println(string(j))
			return
		}

		running := 0
		stopped := 0
		errors := 0

		var data [][]string
		for _, child := range children {
			status := "UNKNOWN"
			uptime := "-"
			url := "-"

			if child.EnvId != "" {
				s, err := rdeGetEnvStatus(client, child.EnvId)
				if err == nil {
					status = string(utils.GetStatusTextWithColor(s))
					if s == qovery.STATEENUM_DEPLOYED || s == qovery.STATEENUM_RESTARTED {
						running++
						url = rdeGetWorkspaceUrl(client, child.EnvId)
						if url == "" {
							url = "-"
						}
						lastDeploy := rdeGetLastDeployTime(client, child.EnvId)
						uptime = rdeFormatUptime(lastDeploy)
					} else if s == qovery.STATEENUM_STOPPED {
						stopped++
					} else {
						errors++
					}
				} else {
					errors++
				}
			}

			bpName := rdeBlueprintNameForProjectId(client, child.BlueprintProjectId)

			data = append(data, []string{child.ProjectName, bpName, status, uptime, url})
		}

		err = utils.PrintTable([]string{"Name", "Blueprint", "Status", "Uptime", "Workspace URL"}, data)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable")
		}

		utils.Println(fmt.Sprintf("\nTotal: %d RDE(s) (%d running, %d stopped, %d error/other)", len(children), running, stopped, errors))
	},
}

func init() {
	rdeCmd.AddCommand(rdeListCmd)
	rdeListCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name")
	rdeListCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
