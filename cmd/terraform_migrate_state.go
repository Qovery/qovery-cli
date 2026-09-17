package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var terraformMigrateStateCmd = &cobra.Command{
	Use:   "migrate-state",
	Short: "Migrate terraform state to new backend",
	Long: `Migrate terraform state to a new backend configuration.

This command will execute 'terraform init -migrate-state' on the specified
terraform service(s). Use this when changing backend configuration (e.g.,
moving from local state to S3, or changing S3 bucket).

Make sure to update your terraform backend configuration before running
this command.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateTerraformArguments(terraformName, terraformNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// migrate terraform state
		terraformList := buildTerraformListFromTerraformNames(client, envId, terraformName, terraformNames)
		action := "MIGRATE_STATE"
		err := utils.DeployTerraforms(client, envId, terraformList, terraformCommitId, &action)
		utils.CheckError(err)
		utils.Println(fmt.Sprintf("Request to migrate terraform(s) %s state has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		WatchTerraformDeployment(client, envId, terraformList, watchFlag, qovery.STATEENUM_DEPLOYED)
	},
}

func init() {
	terraformCmd.AddCommand(terraformMigrateStateCmd)
	terraformMigrateStateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformMigrateStateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformMigrateStateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformMigrateStateCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformMigrateStateCmd.Flags().StringVarP(&terraformNames, "terraforms", "", "", "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"")
	terraformMigrateStateCmd.Flags().StringVarP(&terraformCommitId, "commit-id", "c", "", "Git Commit ID (optional, defaults to deployed commit)")
	terraformMigrateStateCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch terraform status until it's ready or an error occurs")
}
