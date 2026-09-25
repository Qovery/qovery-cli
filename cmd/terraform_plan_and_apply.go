package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var terraformPlanAndApplyCmd = &cobra.Command{
	Use:   "plan-and-apply",
	Short: "Deploy terraform (plan and apply)",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateTerraformArguments(terraformName, terraformNames)
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// deploy multiple terraforms
		terraformList := buildTerraformListFromTerraformNames(client, envId, terraformName, terraformNames)
		watch := utils.NewServicesWatch(client, envId, terraformIds(terraformList), watchFlag)
		err := utils.DeployTerraforms(client, envId, terraformList, terraformCommitId, nil)
		utils.CheckError(err)
		utils.Println(fmt.Sprintf("Request to deploy terraform(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", terraformName, terraformNames)))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

func buildTerraformListFromTerraformNames(
	client *qovery.APIClient,
	environmentId string,
	terraformName string,
	terraformNames string,
) []*qovery.TerraformResponse {
	var terraformList []*qovery.TerraformResponse
	terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), environmentId).Execute()
	utils.CheckError(err)

	if terraformName != "" {
		terraform := utils.FindByTerraformName(terraforms.GetResults(), terraformName)
		if terraform == nil {
			utils.PrintlnError(fmt.Errorf("terraform %s not found", terraformName))
			utils.PrintlnInfo("You can list all terraforms with: qovery terraform list")
			os.Exit(1)
		}
		terraformList = append(terraformList, terraform)
	}
	if terraformNames != "" {
		for _, name := range strings.Split(terraformNames, ",") {
			trimmedName := strings.TrimSpace(name)
			terraform := utils.FindByTerraformName(terraforms.GetResults(), trimmedName)
			if terraform == nil {
				utils.PrintlnError(fmt.Errorf("terraform %s not found", name))
				utils.PrintlnInfo("You can list all terraforms with: qovery terraform list")
				os.Exit(1)
			}
			terraformList = append(terraformList, terraform)
		}
	}

	return terraformList
}

func validateTerraformArguments(terraformName string, terraformNames string) {
	if terraformName == "" && terraformNames == "" {
		utils.PrintlnError(fmt.Errorf("use either --terraform \"<terraform name>\" or --terraforms \"<terraform1 name>, <terraform2 name>\" but not both at the same time"))
		os.Exit(1)
	}

	if terraformName != "" && terraformNames != "" {
		utils.PrintlnError(fmt.Errorf("you can't use --terraform and --terraforms at the same time"))
		os.Exit(1)
	}
}

func init() {
	terraformCmd.AddCommand(terraformPlanAndApplyCmd)
	terraformPlanAndApplyCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	terraformPlanAndApplyCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	terraformPlanAndApplyCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	terraformPlanAndApplyCmd.Flags().StringVarP(&terraformName, "terraform", "n", "", "Terraform Name")
	terraformPlanAndApplyCmd.Flags().StringVarP(&terraformNames, "terraforms", "", "", "Terraform Names (comma separated) Example: --terraforms \"tf1,tf2,tf3\"")
	terraformPlanAndApplyCmd.Flags().StringVarP(&terraformCommitId, "commit-id", "c", "", "Git Commit ID, or 'latest' for the newest commit of the branch (optional, defaults to deployed commit)")
	terraformPlanAndApplyCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch terraform status until it's ready or an error occurs")
}
