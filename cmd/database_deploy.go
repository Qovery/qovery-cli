package cmd

import (
	"fmt"
	"github.com/qovery/qovery-client-go"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a database",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		checkError(err)

		validateDatabaseArguments(databaseName, databaseNames)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		checkError(err)

		databaseList := buildDatabaseListFromDatabaseNames(client, envId, databaseName, databaseNames)

		// deploy multiple services
		err = utils.DeployDatabases(client, envId, databaseList)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy database(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", databaseName, databaseNames)))

		if watchFlag {
			time.Sleep(3 * time.Second) // wait for the deployment request to be processed (prevent from race condition)
			if len(databaseList) == 1 {
				utils.WatchDatabase(databaseList[0].Id, envId, client)
			} else {
				utils.WatchEnvironment(envId, qovery.STATEENUM_DEPLOYED, client)
			}
		}

	},
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
