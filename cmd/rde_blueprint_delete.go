package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rdeBlueprintDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Unregister a project as an RDE blueprint",
	Long: `Remove the BLUEPRINT_PROJECT_ID and BLUEPRINT_KEY environment variables
from the project and its environment. This does NOT delete the project itself.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// Delete project-level BLUEPRINT_PROJECT_ID var
		utils.Println(fmt.Sprintf("Removing %s from project %s...", rdeBlueprintProjectIdVar, bp.ProjectName))
		projectVars, err := utils.ListProjectVariables(client, bp.ProjectId)
		if err == nil {
			bpVar := utils.FindEnvironmentVariableByKey(rdeBlueprintProjectIdVar, projectVars)
			if bpVar != nil {
				_, err = client.VariableMainCallsAPI.DeleteVariable(ctx(), bpVar.Id).Execute()
				if err != nil {
					utils.PrintlnError(fmt.Errorf("failed to delete %s: %w", rdeBlueprintProjectIdVar, err))
				}
			}
		}

		// Delete environment-level BLUEPRINT_KEY var
		if bp.EnvId != "" {
			utils.Println(fmt.Sprintf("Removing %s from environment %s...", rdeBlueprintKeyVar, bp.EnvName))
			envVars, err := utils.ListEnvironmentVariables(client, bp.EnvId)
			if err == nil {
				bkVar := utils.FindEnvironmentVariableByKey(rdeBlueprintKeyVar, envVars)
				if bkVar != nil {
					_, err = client.VariableMainCallsAPI.DeleteVariable(ctx(), bkVar.Id).Execute()
					if err != nil {
						utils.PrintlnError(fmt.Errorf("failed to delete %s: %w", rdeBlueprintKeyVar, err))
					}
				}
			}
		}

		utils.Println("")
		utils.Println(fmt.Sprintf("Blueprint %s unregistered. Project and environments are preserved.", bp.ProjectName))
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintDeleteCmd)
	rdeBlueprintDeleteCmd.Flags().StringVarP(&rdeBlueprintProjectName, "project", "p", "", "Blueprint Project Name to unregister")
	rdeBlueprintDeleteCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeBlueprintDeleteCmd.MarkFlagRequired("project")
}
