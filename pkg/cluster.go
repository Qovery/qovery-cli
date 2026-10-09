package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

func GetKubeconfigByClusterId(clusterId string, readOnly bool) string {
	qoveryClient := GetQoveryClientInstance()

	request := qoveryClient.ClustersAPI.GetClusterKubeconfig(
		context.Background(),
		"00000000-0000-0000-000000000000",
		clusterId,
	).WithTokenFromCli(true)

	if readOnly {
		request = request.ReadOnly(true)
	}

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

func GetKubeconfigByClusterIdWithError(clusterId string, readOnly bool) (string, error) {
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		return "", fmt.Errorf("failed to get Qovery access token: %w", err)
	}

	qoveryClient := utils.GetQoveryClient(tokenType, token)
	request := qoveryClient.ClustersAPI.GetClusterKubeconfig(
		context.Background(),
		"00000000-0000-0000-0000-000000000000",
		clusterId,
	).WithTokenFromCli(true)
	if readOnly {
		request = request.ReadOnly(true)
	}

	response, httpResponse, err := qoveryClient.ClustersAPI.GetClusterKubeconfigExecute(request)
	if err != nil {
		return "", fmt.Errorf("failed to fetch cluster kubeconfig: %w", err)
	}
	if httpResponse == nil {
		return "", errors.New("missing HTTP response while fetching cluster kubeconfig")
	}
	if httpResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cannot fetch cluster kubeconfig (status_code=%d)", httpResponse.StatusCode)
	}
	return response, nil
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

func GetTokenByClusterId(clusterId string, readOnly bool) string {
	qoveryClient := GetQoveryClientInstance()

	request := qoveryClient.DefaultAPI.GetClusterTokenByClusterId(context.Background(), clusterId)
	if readOnly {
		request = request.ReadOnly(true)
	}
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
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	return utils.GetQoveryClient(tokenType, token)
}
