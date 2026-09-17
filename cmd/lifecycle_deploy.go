package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var lifecycleDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a lifecycle job",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateLifecycleArguments(lifecycleName, lifecycleNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		if lifecycleTag != "" && lifecycleCommitId != "" {
			utils.PrintlnError(fmt.Errorf("you can't use --tag and --commit-id at the same time"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		lifecyleList := buildLifecycleListFromLifecycleNames(client, envId, lifecycleName, lifecycleNames)
		err := utils.DeployJobs(client, envId, lifecyleList, lifecycleCommitId, lifecycleTag)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy lifecycle job(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", lifecycleName, lifecycleNames)))
		WatchJobDeployment(client, envId, lifecyleList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	lifecycleCmd.AddCommand(lifecycleDeployCmd)
	lifecycleDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	lifecycleDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	lifecycleDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleName, "lifecycle", "n", "", "Lifecycle Job Name")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleNames, "lifecycles", "", "", "Lifecycle Job Names")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleCommitId, "commit-id", "c", "", "Lifecycle Commit ID")
	lifecycleDeployCmd.Flags().StringVarP(&lifecycleTag, "tag", "t", "", "Lifecycle Tag")
	lifecycleDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch lifecycle status until it's ready or an error occurs")
}
