package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/qovery/qovery-cli/utils"
)

// RequireExtendedCredentials requires extended credentials to deploy clusters.
// Without targetClusterId, it applies to all clusters (optionally only the production ones).
// With targetClusterId, it only applies to this cluster and production must be false.
func RequireExtendedCredentials(production bool, targetClusterId string) error {
	if targetClusterId == "" {
		return requireExtendedCredentialsForAllClusters(production)
	}

	if production {
		return fmt.Errorf("--production cannot be used with --target-cluster-id")
	}
	clusterId, err := uuid.Parse(targetClusterId)
	if err != nil {
		return fmt.Errorf("invalid --target-cluster-id: %w", err)
	}
	return requireExtendedCredentialsForCluster(clusterId.String())
}

func requireExtendedCredentialsForAllClusters(production bool) error {
	body, err := json.Marshal(map[string]bool{"production": production})
	if err != nil {
		return err
	}

	res, err := postWithBody(utils.GetAdminUrl()+"/cluster/requireWideAccessToDeploy", string(body))
	if err != nil {
		return err
	}

	result, err := readRequireExtendedCredentialsResponse(res, "could not require extended credentials")
	if err != nil {
		return err
	}

	utils.Println(fmt.Sprintf("Extended credentials required (production=%t): %s", production, result))
	return nil
}

func requireExtendedCredentialsForCluster(clusterId string) error {
	res, err := postWithBody(utils.GetAdminUrl()+"/cluster/"+clusterId+"/requireWideAccessToDeploy", "")
	if err != nil {
		return err
	}

	result, err := readRequireExtendedCredentialsResponse(res, fmt.Sprintf("could not require extended credentials for cluster %s", clusterId))
	if err != nil {
		return err
	}

	utils.Println(fmt.Sprintf("Extended credentials required for cluster %s: %s", clusterId, result))
	return nil
}

func readRequireExtendedCredentialsResponse(res *http.Response, errorPrefix string) (string, error) {
	defer func() { _ = res.Body.Close() }()

	result, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("%s (error %s: %s)", errorPrefix, res.Status, string(result))
	}
	return string(result), nil
}
