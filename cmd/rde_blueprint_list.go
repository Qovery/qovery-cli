package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rdeBlueprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all RDE blueprints",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		blueprints, err := rdeListBlueprintProjects(client, orgId)
		checkError(err)

		if len(blueprints) == 0 {
			utils.Println("No RDE blueprints found.")
			return
		}

		if jsonFlag {
			var results []interface{}
			for _, bp := range blueprints {
				status := ""
				if bp.EnvId != "" {
					s, err := rdeGetEnvStatus(client, bp.EnvId)
					if err == nil {
						status = string(s)
					}
				}
				results = append(results, map[string]interface{}{
					"project_id":   bp.ProjectId,
					"project_name": bp.ProjectName,
					"env_id":       bp.EnvId,
					"env_name":     bp.EnvName,
					"status":       status,
				})
			}
			j, _ := json.Marshal(results)
			utils.Println(string(j))
			return
		}

		var data [][]string
		for _, bp := range blueprints {
			status := "NO_ENV"
			if bp.EnvId != "" {
				s, err := rdeGetEnvStatus(client, bp.EnvId)
				if err == nil {
					status = string(utils.GetStatusTextWithColor(s))
				}
			}

			envName := "-"
			if bp.EnvName != "" {
				envName = bp.EnvName
			}

			data = append(data, []string{bp.ProjectName, envName, status, bp.ProjectId})
		}

		err = utils.PrintTable([]string{"Project Name", "Environment", "Status", "Project ID"}, data)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("\nTotal: %d blueprint(s)", len(blueprints)))
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintListCmd)
	rdeBlueprintListCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeBlueprintListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
