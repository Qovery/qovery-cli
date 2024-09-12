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

var cronjobDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a cronjob",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobName == "" && cronjobNames == "" {
			utils.PrintlnError(fmt.Errorf("use either --cronjob \"<cronjob name>\" or --cronjobs \"<cronjob1 name>, <cronjob2 name>\" but not both at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if cronjobName != "" && cronjobNames != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --cronjob and --cronjobs at the same time"))
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

		if cronjobNames != "" {
			// wait until service is ready
			for {
				if utils.IsEnvironmentInATerminalState(envId, client) {
					break
				}

				utils.Println(fmt.Sprintf("Waiting for environment %s to be ready..", pterm.FgBlue.Sprintf("%s", envId)))
				time.Sleep(5 * time.Second)
			}

			cronjobs, err := ListCronjobs(envId, client)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			var serviceIds []string
			for _, cronjobName := range strings.Split(cronjobNames, ",") {
				trimmedCronjobName := strings.TrimSpace(cronjobName)
				serviceIds = append(serviceIds, utils.GetJobId(utils.FindByJobName(cronjobs, trimmedCronjobName)))
			}

			_, err = utils.DeleteServices(client, envId, serviceIds, utils.JobType)

			if watchFlag {
				utils.WatchEnvironment(envId, "unused", client)
			} else {
				utils.Println(fmt.Sprintf("Deleting cronjobs %s in progress..", pterm.FgBlue.Sprintf("%s", cronjobNames)))
			}

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			return
		}

		cronjobs, err := ListCronjobs(envId, client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		job := utils.FindByJobName(cronjobs, cronjobName)

		if job == nil {
			utils.PrintlnError(fmt.Errorf("job %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		msg, err := utils.DeleteService(client, envId, utils.GetJobId(job), utils.JobType, watchFlag)

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
			utils.Println(fmt.Sprintf("Cronjob %s deleted!", pterm.FgBlue.Sprintf("%s", cronjobName)))
		} else {
			utils.Println(fmt.Sprintf("Deleting cronjob %s in progress..", pterm.FgBlue.Sprintf("%s", cronjobName)))
		}
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobDeleteCmd)
	cronjobDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobDeleteCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobDeleteCmd.Flags().StringVarP(&cronjobNames, "cronjobs", "", "", "Cronjob Names (comma separated) (ex: --cronjobs \"cron1,cron2\")")
	cronjobDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cronjob status until it's ready or an error occurs")
}
