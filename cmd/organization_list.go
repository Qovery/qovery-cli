package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var organizationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations the authenticated token has access to",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		organizations, _, err := client.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getOrganizationJsonOutput(organizations.GetResults()))
			return
		}

		var data [][]string

		for _, organization := range organizations.GetResults() {
			data = append(data, []string{organization.Id, organization.GetName(), string(organization.GetPlan())})
		}

		err = utils.PrintTable([]string{"Id", "Name", "Plan"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getOrganizationJsonOutput(organizations []qovery.Organization) string {
	organizationJSON, err := json.Marshal(organizations)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(organizationJSON)
}

func init() {
	organizationCmd.AddCommand(organizationListCmd)
	organizationListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
