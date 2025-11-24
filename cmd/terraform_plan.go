package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var terraformPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Run terraform plan (dry-run)",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateTerraformArguments(terraformName, terraformNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// plan terraform
		terraformList := buildTerraformListFromTerraformNames(client, envId, terraformName, terraformNames)
		action := "PLAN"
		err := utils.DeployTerraforms(client, envId, terraformList, terraformCommitId, &action)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to plan terraform(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		WatchTerraformDeployment(client, envId, terraformList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	terraformCmd.AddCommand(terraformPlanCmd)
	terraformPlanCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformPlanCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformPlanCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformPlanCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformPlanCmd.Flags().StringVarP(&terraformNames, "terraforms", "", "", "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"")
	terraformPlanCmd.Flags().StringVarP(&terraformCommitId, "commit-id", "c", "", "Git Commit ID (optional, defaults to deployed commit)")
	terraformPlanCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch terraform status until it's ready or an error occurs")
}
