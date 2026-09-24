package cmd

import (
	"fmt"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show RDE platform overview",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		// Get organization name
		orgName := orgId
		orgs, _, err := client.OrganizationMainCallsAPI.ListOrganization(ctx()).Execute()
		if err == nil {
			for _, org := range orgs.GetResults() {
				if org.Id == orgId {
					orgName = org.Name
					break
				}
			}
		}

		// List blueprints
		blueprints, _ := rdeListBlueprintProjects(client, orgId)

		// List all children and count statuses
		allChildren, _ := rdeListAllChildren(client, orgId)
		running := 0
		stopped := 0
		errors := 0

		for _, child := range allChildren {
			if child.EnvId != "" {
				status, err := rdeGetEnvStatus(client, child.EnvId)
				if err != nil {
					errors++
					continue
				}
				switch status {
				case qovery.STATEENUM_DEPLOYED, qovery.STATEENUM_RESTARTED:
					running++
				case qovery.STATEENUM_STOPPED:
					stopped++
				default:
					errors++
				}
			}
		}

		// Platform summary table
		rdePrintKeyValueTable([][]string{
			{"Organization", fmt.Sprintf("%s (%s)", orgName, orgId)},
			{"Blueprints", fmt.Sprintf("%d", len(blueprints))},
			{"RDEs", fmt.Sprintf("%d total (%d running, %d stopped, %d error/other)", len(allChildren), running, stopped, errors)},
		})

		// Blueprints detail table
		if len(blueprints) > 0 {
			utils.Println("")
			var data [][]string
			for _, bp := range blueprints {
				status := "NO_ENV"
				if bp.EnvId != "" {
					s, err := rdeGetEnvStatus(client, bp.EnvId)
					if err == nil {
						status = utils.GetStatusTextWithColor(s)
					}
				}
				data = append(data, []string{bp.ProjectName, bp.EnvName, status, bp.ProjectId})
			}
			_ = utils.PrintTable([]string{"Blueprint", "Environment", "Status", "Project ID"}, data)
		}
	},
}

func init() {
	rdeCmd.AddCommand(rdeInfoCmd)
	rdeInfoCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
}
