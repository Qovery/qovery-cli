package cmd

import (
	"context"
	"github.com/go-errors/errors"
	"io"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		orgId, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := qovery.CloneEnvironmentRequest{
			Name:                newEnvironmentName,
			ApplyDeploymentRule: &applyDeploymentRule,
		}

		if clusterName != "" {
			clusters, _, err := client.ClustersAPI.ListOrganizationCluster(context.Background(), orgId).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			if err == nil {
				for _, c := range clusters.GetResults() {
					if strings.EqualFold(c.Name, clusterName) {
						req.ClusterId = &c.Id
						break
					}
				}
			}
		}

		if environmentType != "" {
			switch strings.ToUpper(environmentType) {
			case "DEVELOPMENT":
				req.Mode = qovery.EnvironmentModeEnum.Ptr(qovery.ENVIRONMENTMODEENUM_DEVELOPMENT)
			case "PRODUCTION":
				req.Mode = qovery.EnvironmentModeEnum.Ptr(qovery.ENVIRONMENTMODEENUM_PRODUCTION)
			case "STAGING":
				req.Mode = qovery.EnvironmentModeEnum.Ptr(qovery.ENVIRONMENTMODEENUM_STAGING)
			}
		}

		if targetProjectName != "" {
			targetProjectId, err := getProjectContextResourceId(client, targetProjectName, orgId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			req.ProjectId = &targetProjectId
		}

		_, res, err := client.EnvironmentActionsAPI.CloneEnvironment(context.Background(), envId).CloneEnvironmentRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res != nil && !strings.Contains(res.Status, "200") {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Environment is cloned!")
	},
}

func init() {
	environmentCmd.AddCommand(environmentCloneCmd)
	environmentCloneCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	environmentCloneCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project Name")
	environmentCloneCmd.Flags().StringVarP(&environmentName, "environment", "e", "", "Environment Name to clone")
	environmentCloneCmd.Flags().StringVarP(&newEnvironmentName, "new-environment-name", "n", "", "New Environment Name")
	environmentCloneCmd.Flags().StringVarP(&clusterName, "cluster", "c", "", "Cluster Name where to clone the environment")
	environmentCloneCmd.Flags().StringVarP(&environmentType, "environment-type", "t", "", "Environment type for the new environment (DEVELOPMENT|STAGING|PRODUCTION)")
	environmentCloneCmd.Flags().BoolVarP(&applyDeploymentRule, "apply-deployment-rule", "", false, "Enable applying deployment rules on the new environment instead of having a pristine clone. Default: false")
	environmentCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")

	_ = environmentCloneCmd.MarkFlagRequired("new-environment-name")
}
