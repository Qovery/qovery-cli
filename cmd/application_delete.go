package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if applicationName == "" && applicationNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --application \"<app name>\" or --applications \"<app1 name>, <app2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if applicationName != "" && applicationNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --application and --applications at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if applicationNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, applicationName := range strings.Split(applicationNames, ",") {
				trimmedApplicationName := strings.TrimSpace(applicationName)
				serviceIds = append(serviceIds, utils.FindByApplicationName(applications.GetResults(), trimmedApplicationName).Id)
			}

			// stop multiple services
			_, err = utils.DeleteServices(client, envId, serviceIds, utils.ApplicationType)
			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Deleting applications %s in progress..", pterm.FgBlue.Sprintf("%s", applicationNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.DeleteService(client, envId, application.Id, utils.ApplicationType, watchFlag)

		if watchFlag {
			utils.WatchEnvironment(envId, "unused", client)
		}

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if msg != "" {
			utils.PrintlnInfo(msg)
			return
		}

		if watchFlag {
			utils.Println(fmt.Sprintf("Application %s deleted!", pterm.FgBlue.Sprintf("%s", applicationName)))
		} else {
			utils.Println(fmt.Sprintf("Deleting application %s in progress..", pterm.FgBlue.Sprintf("%s", applicationName)))
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationDeleteCmd)
	applicationDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDeleteCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDeleteCmd.Flags().StringVarP(&applicationNames, "applications", "", "", "Application Names (comma separated) Example: --applications \"app1,app2,app3\"")
	applicationDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch application status until it's ready or an error occurs")
}
