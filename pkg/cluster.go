package pkg

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

func GetKubeconfigByClusterId(clusterId string) string {
	qoveryClient := GetQoveryClientInstance()

	request := qoveryClient.ClustersAPI.GetClusterKubeconfig(
		context.Background(),
		"00000000-0000-0000-000000000000",
		clusterId,
	).WithTokenFromCli(true)
	response, httpResponse, err := qoveryClient.ClustersAPI.GetClusterKubeconfigExecute(request)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	if httpResponse.StatusCode != 200 {
		utils.PrintlnInfo(fmt.Sprintf("cannot fetch cluster token (status_code=%d)", httpResponse.StatusCode))
		os.Exit(1)
	}
	return response
}

func UpdateClusterKubeconfig(organizationId string, clusterId string, kubeconfig string) error {
	qoveryClient := GetQoveryClientInstance()

	request := qoveryClient.ClustersAPI.EditClusterKubeconfig(
		context.Background(),
		organizationId,
		clusterId,
	).Body(kubeconfig)

	// Execute the request
	response, err := request.Execute()
	if err != nil {
		utils.PrintlnError(err)
		return err
	}
	defer func() { _ = response.Body.Close() }()

	return nil
}

func GetTokenByClusterId(clusterId string) string {
	qoveryClient := GetQoveryClientInstance()

	request := qoveryClient.DefaultAPI.GetClusterTokenByClusterId(context.Background(), clusterId)
	_, response, err := qoveryClient.DefaultAPI.GetClusterTokenByClusterIdExecute(request)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	if response.StatusCode != 200 {
		utils.PrintlnInfo(fmt.Sprintf("cannot fetch cluster token (status_code=%d)", response.StatusCode))
		os.Exit(1)
	}
	body, _ := io.ReadAll(response.Body)
	return string(body)
}

func GetQoveryClientInstance() *qovery.APIClient {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	return utils.GetQoveryClient(tokenType, token)
}
