package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		statuses, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		var data [][]string

		for _, application := range applications.GetResults() {
			data = append(data, []string{*application.Name, "Application",
				utils.GetStatus(statuses.GetApplications(), application.Id), application.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationListCmd)
	applicationListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
}
