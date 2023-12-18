package cmd

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/pterm/pterm"
	"io"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var helmCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm, err := getHelmContextResource(client, helmName, envId)

		if err != nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		targetProjectId := projectId // use same project as the source project
		if targetProjectName != "" {

			targetProjectId, err = getProjectContextResourceId(client, targetProjectName, organizationId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		targetEnvironmentId := envId // use same env as the source env
		if targetEnvironmentName != "" {

			targetEnvironmentId, err = getEnvironmentContextResourceId(client, targetEnvironmentName, targetProjectId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		if targetHelmName == "" {
			// use same helm name as the source helm
			targetHelmName = helm.Name
		}

		req := qovery.CloneServiceRequest{
			Name:          targetHelmName,
			EnvironmentId: targetEnvironmentId,
		}

		clonedService, res, err := client.HelmsAPI.CloneHelm(context.Background(), helm.Id).CloneServiceRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		name := ""
		if clonedService != nil {
			name = clonedService.Name
		}

		utils.Println(fmt.Sprintf("Helm %s cloned!", pterm.FgBlue.Sprintf(name)))
	},
}


func init() {
	helmCmd.AddCommand(helmCloneCmd)
	helmCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmCloneCmd.Flags().StringVarP(&helmName, "helm", "n", "", "Helm Name")
	helmCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")
	helmCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	helmCloneCmd.Flags().StringVarP(&targetHelmName, "target-helm-name", "", "", "Target Helm Name")

	_ = helmCloneCmd.MarkFlagRequired("helm")
}
