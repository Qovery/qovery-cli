package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var terraformForceUnlockCmd = &cobra.Command{
	Use:   "force-unlock",
	Short: "Force unlock terraform state file",
	Long: `Force unlock terraform state file when it's stuck.

This command will execute 'terraform force-unlock' on the specified terraform service(s).
Use this when a state lock is preventing operations and you're certain no other
operations are running.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateTerraformArguments(terraformName, terraformNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// force unlock terraform state
		terraformList := buildTerraformListFromTerraformNames(client, envId, terraformName, terraformNames)
		action := "FORCE_UNLOCK"
		err := utils.DeployTerraforms(client, envId, terraformList, terraformCommitId, &action)
		utils.CheckError(err)
		utils.Println(fmt.Sprintf("Request to force unlock terraform(s) %s state has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		WatchTerraformDeployment(client, envId, terraformList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	terraformCmd.AddCommand(terraformForceUnlockCmd)
	terraformForceUnlockCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformForceUnlockCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformForceUnlockCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformForceUnlockCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformForceUnlockCmd.Flags().StringVarP(&terraformNames, "terraforms", "", "", "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"")
	terraformForceUnlockCmd.Flags().StringVarP(&terraformCommitId, "commit-id", "c", "", "Git Commit ID (optional, defaults to deployed commit)")
	terraformForceUnlockCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch terraform status until it's ready or an error occurs")
}
