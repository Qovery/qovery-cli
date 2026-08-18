package cmd

import (
	"context"
	"fmt"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	qovery "github.com/qovery/qovery-client-go"
)

type operatorCommandContext struct {
	api            *qovery.APIClient
	clusterID      string
	organizationID string
}

func newOperatorCommandContext(organizationName string, clusterName string) (*operatorCommandContext, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := utils.GetQoveryClient(tokenType, token)
	organizationID, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	if err != nil {
		return nil, err
	}

	clusters, _, err := client.ClustersAPI.ListOrganizationCluster(context.Background(), organizationID).Execute()
	if err != nil {
		return nil, err
	}
	cluster := findCluster(clusters.GetResults(), clusterName)
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	return &operatorCommandContext{
		api:            client,
		clusterID:      cluster.Id,
		organizationID: organizationID,
	}, nil
}

func findCluster(clusters []qovery.Cluster, name string) *qovery.Cluster {
	for index := range clusters {
		if clusters[index].Name == name {
			return &clusters[index]
		}
	}
	return nil
}

func displayVersion(version qovery.NullableString) string {
	value := version.Get()
	if value == nil || *value == "" {
		return "unknown"
	}
	return *value
}
