package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleName == "" && lifecycleNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --lifecycle \"<lifecycle name>\" or --lifecycles \"<lifecycle1 name>, <lifecycle2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if lifecycleName != "" && lifecycleNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --lifecycle and --lifecycles at the same time"))
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

		if lifecycleNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
				time.Sleep(5 * time.Second)
			}

			lifecycles, err := ListLifecycleJobs(envId, client)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, lifecycleName := range strings.Split(lifecycleNames, ",") {
				trimmedLifecycleName := strings.TrimSpace(lifecycleName)
				serviceIds = append(serviceIds, utils.FindByJobName(lifecycles, trimmedLifecycleName).LifecycleJobResponse.Id)
			}

			// stop multiple services
			_, err = utils.DeleteServices(client, envId, serviceIds, utils.JobType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Deleting lifecycle jobs %s in progress..", pterm.FgBlue.Sprintf(lifecycleNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		lifecycles, err := ListLifecycleJobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecycle := utils.FindByJobName(lifecycles, lifecycleName)

		if lifecycle == nil || lifecycle.LifecycleJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("lifecycle %s not found", lifecycleName))
			utils.PrintlnInfo("You can list all lifecycle jobs with: qovery lifecycle list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.DeleteService(client, envId, lifecycle.LifecycleJobResponse.Id, utils.JobType, watchFlag)

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
			utils.Println(fmt.Sprintf("Lifecycle %s deleted!", pterm.FgBlue.Sprintf(lifecycleName)))
		} else {
			utils.Println(fmt.Sprintf("Deleting lifecycle %s in progress..", pterm.FgBlue.Sprintf(lifecycleName)))
		}
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeleteCmd)
	lifecycleDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeleteCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeleteCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle Job Names (comma separated) (ex: --lifecycles \"lifecycle1,lifecycle2\")")
	lifecycleDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle job status until it's ready or an error occurs")
}
