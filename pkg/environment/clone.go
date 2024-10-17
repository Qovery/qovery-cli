package environment

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/go-errors/errors"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

func EnvironmentClone(client *qovery.APIClient, organizationName string, projectName string, environmentName string, newEnvironmentName string, clusterName string, environmentType string, applyDeploymentRule bool, orgId string, envId string) *qovery.Environment {

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

	cloneRequestAnswer, res, err := client.EnvironmentActionsAPI.CloneEnvironment(context.Background(), envId).CloneEnvironmentRequest(req).Execute()

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

	return cloneRequestAnswer
}
