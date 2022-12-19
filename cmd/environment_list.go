package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		_, projectId, _, err := getContextResourcesId(auth, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		environments, _, err := client.EnvironmentsApi.ListEnvironment(auth, projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		statuses, _, err := client.EnvironmentsApi.GetProjectEnvironmentsStatus(auth, projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		var data [][]string

		for _, env := range environments.GetResults() {
			data = append(data, []string{env.GetName(), *env.ClusterName, string(env.Mode),
				utils.GetStatus(statuses.GetResults(), env.Id), env.UpdatedAt.String()})
		}

		err = utils.PrintTable([]string{"Name", "Cluster", "Type", "Status", "Last Update"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	environmentCmd.AddCommand(environmentListCmd)
	environmentListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
}
