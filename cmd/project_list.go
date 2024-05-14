package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, err := getOrganizationContextResourceId(client, organizationName)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		projects, _, err := client.ProjectsAPI.ListProject(context.Background(), organizationId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(getProjectJsonOutput(projects.GetResults()))
			return
		}

		var data [][]string

		for _, project := range projects.GetResults() {
			data = append(data, []string{project.Id, project.GetName()})
		}

		err = utils.PrintTable([]string{"Id", "Name"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getProjectJsonOutput(projects []qovery.Project) string {
	projectJSON, err := json.Marshal(projects)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(projectJSON)
}

func init() {
	projectCmd.AddCommand(projectListCmd)
	projectListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	projectListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
