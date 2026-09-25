package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a database without redeploying it",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateDatabaseArguments(databaseName, databaseNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		databaseList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)
		watch := utils.NewServicesWatch(client, envId, databaseIds(databaseList), watchFlag)
		err := utils.RebootServices(client, envId, qovery.RebootServicesRequest{DatabaseIds: databaseIds(databaseList)})
		checkError(err)
		utils.Println(fmt.Sprintf("Request to restart database(s) %s has been queued...", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
		watch.Wait(qovery.STATEENUM_RESTARTED)
	},
}

func init() {
	databaseCmd.AddCommand(databaseRestartCmd)
	databaseRestartCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseRestartCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseRestartCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseRestartCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseRestartCmd.Flags().StringVarP(&databaseNames, "databases", "", "", "Database Names (comma separated) Example: --databases \"db1,db2\"")
	databaseRestartCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")
}
