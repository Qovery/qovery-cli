package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if helmName == "" && helmNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --helm \"<helm name>\" or --helms \"<helm1 name>, <helm2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if helmName != "" && helmNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --helm and --helms at the same time"))
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

		if helmNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf(envId)))
				time.Sleep(5 * time.Second)
			}

			helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, helmName := range strings.Split(helmNames, ",") {
				trimmedHelmName := strings.TrimSpace(helmName)

				helm := utils.FindByHelmName(helms.GetResults(), trimmedHelmName)
				if helm == nil {
					utils.PrintlnError(fmt.Errorf("helm %s not found", trimmedHelmName))
					utils.PrintlnInfo("You can list all helms with: qovery helm list")
					os.Exit(1)
					panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
				}
				serviceIds = append(serviceIds, helm.Id)
			}

			// stop multiple services
			_, err = utils.StopServices(client, envId, serviceIds, utils.HelmType)

			if watchFlag {
				utils.WatchEnvironment(envId, qovery.STATEENUM_STOPPED, client)
			} else {
				utils.Println(fmt.Sprintf("Stopping helms %s in progress..", pterm.FgBlue.Sprintf(helmNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm := utils.FindByHelmName(helms.GetResults(), helmName)

		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.StopService(client, envId, helm.Id, utils.HelmType, watchFlag)

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
			utils.Println(fmt.Sprintf("Helm %s stopped!", pterm.FgBlue.Sprintf(helmName)))
		} else {
			utils.Println(fmt.Sprintf("Stopping helm %s in progress..", pterm.FgBlue.Sprintf(helmName)))
		}
	},
}

func init() {
	helmCmd.AddCommand(helmStopCmd)
	helmStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmStopCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmStopCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmStopCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmStopCmd.Flags().StringVarP(&helmNames, "helms", "", "", "Helm Names (comma separated) (ex: --helms \"helm1,helm2\")")
	helmStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch helm status until it's ready or an error occurs")
}
