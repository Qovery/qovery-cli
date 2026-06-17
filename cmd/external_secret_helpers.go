package cmd

import (
	"context"
	"fmt"

	"github.com/qovery/qovery-client-go"
	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
)

func getSecretManagerAccessIdByName(client *qovery.APIClient, organizationId, envId, name string) (string, error) {
	env, _, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), envId).Execute()
	if err != nil {
		return "", fmt.Errorf("failed to get environment: %w", err)
	}

	clusters, err := cluster.NewClusterService(client, &promptuifactory.PromptUiFactoryImpl{}).ListClusters(organizationId)
	if err != nil {
		return "", fmt.Errorf("failed to list clusters: %w", err)
	}

	var matchedCluster *qovery.Cluster
	for i, c := range clusters.GetResults() {
		if c.Id == env.ClusterId {
			matchedCluster = &clusters.GetResults()[i]
			break
		}
	}
	if matchedCluster == nil {
		return "", fmt.Errorf("cluster %s not found in organization", env.ClusterId)
	}

	for _, sma := range matchedCluster.SecretManagerAccesses {
		if sma.Name == name {
			return sma.Id, nil
		}
	}
	return "", fmt.Errorf("secret manager access %q not found in cluster %s", name, matchedCluster.Name)
}
