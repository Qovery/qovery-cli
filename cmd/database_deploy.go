package cmd

import (
	"fmt"
	"time"

	"github.com/qovery/qovery-client-go"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.ShowHelpIfNoArgs(cmd, args)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateDatabaseArguments(databaseName, databaseNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		databaseList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)
		err := utils.DeployDatabases(client, envId, databaseList)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy database(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))
		WatchDatabaseDeployment(client, envId, databaseList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func WatchDatabaseDeployment(
	client *qovery.APIClient,
	envId string,
	databaseList []*qovery.Database,
	watchFlag bool,
	finalServiceState qovery.StateEnum,
) {
	if watchFlag {
		time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
		if len(databaseList) == 1 {
			utils.WatchDatabase(databaseList[0].Id, envId, client)
		} else {
			utils.WatchEnvironment(envId, finalServiceState, client)
		}
	}
}

func init() {
	databaseCmd.AddCommand(databaseDeployCmd)
	databaseDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseDeployCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseDeployCmd.Flags().StringVarP(&databaseNames, "databases", "", "", "Database Names (comma separated) (ex: --databases \"database1,database2\")")
	databaseDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch database status until it's ready or an error occurs")
}
