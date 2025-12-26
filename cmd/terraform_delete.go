package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var skipDestroyFlag bool
var resourcesOnlyFlag bool

var terraformDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete terraform resources",
	Long: `Delete terraform resources and remove from Qovery.

By default, this will execute 'terraform destroy' to delete all resources
managed by this terraform service, then remove the service from Qovery.

Use --skip-destroy to keep the infrastructure resources but remove the
Qovery configuration. This is useful when you want to manage the resources
outside of Qovery or import them into another system.

Use --resources-only to delete the infrastructure resources but keep the
Qovery configuration. This is useful when you want to clean up resources
while keeping the terraform service definition in Qovery.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateTerraformArguments(terraformName, terraformNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// Validate that skip-destroy and resources-only are mutually exclusive
		if skipDestroyFlag && resourcesOnlyFlag {
			utils.PrintlnError(fmt.Errorf("--skip-destroy and --resources-only flags are mutually exclusive"))
			return
		}

		// delete terraform resources
		terraformList := buildTerraformListFromTerraformNames(client, envId, terraformName, terraformNames)
		err := utils.DeleteTerraforms(client, envId, terraformList, skipDestroyFlag, resourcesOnlyFlag)
		utils.CheckError(err)

		if skipDestroyFlag {
			utils.Println(fmt.Sprintf("Request to remove terraform(s) %s from Qovery (keeping resources) has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		} else if resourcesOnlyFlag {
			utils.Println(fmt.Sprintf("Request to delete resources for terraform(s) %s (keeping Qovery configuration) has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		} else {
			utils.Println(fmt.Sprintf("Request to delete terraform(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		}

		WatchTerraformDeployment(client, envId, terraformList, watchFlag, qovery.STATEENUM_DELETED)
	},
}

func init() {
	terraformCmd.AddCommand(terraformDeleteCmd)
	terraformDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformDeleteCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformDeleteCmd.Flags().StringVarP(&terraformNames, "terraforms", "", "", "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"")
	terraformDeleteCmd.Flags().BoolVarP(&skipDestroyFlag, "skip-destroy", "", false, "Skip terraform destroy (keep resources, only remove from Qovery)")
	terraformDeleteCmd.Flags().BoolVarP(&resourcesOnlyFlag, "resources-only", "", false, "Delete resources only (keep Qovery configuration)")
	terraformDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch terraform status until it's ready or an error occurs")
}
