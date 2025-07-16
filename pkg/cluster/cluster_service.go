package cluster

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"io"
	"time"

	"github.com/go-errors/errors"
	"github.com/pterm/pterm"

	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
)

type ClusterService interface {
	DeployCluster(organizationName string, clusterName string, watchFlag bool) error
	StopCluster(organizationName string, clusterName string, watchFlag bool) error
	ListClusters(organizationId string) (*qovery.ClusterResponseList, error)
	ListClusterRegions(cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterRegionResponseList, error)
	AskToEditStorageClass(cluster *qovery.Cluster) error
}

type ClusterServiceImpl struct {
	client          *qovery.APIClient
	promptUiFactory promptuifactory.PromptUiFactory
}

func NewClusterService(
	client *qovery.APIClient,
	promptUiFactory promptuifactory.PromptUiFactory,
) *ClusterServiceImpl {
	return &ClusterServiceImpl{
		client,
		promptUiFactory,
	}
}

func (service *ClusterServiceImpl) DeployCluster(organizationName string, clusterName string, watchFlag bool) error {
	orgId, err := usercontext.GetOrganizationContextResourceId(service.client, organizationName)

	if err != nil {
		return err
	}

	clusters, _, err := service.client.ClustersAPI.ListOrganizationCluster(context.Background(), orgId).Execute()

	if err != nil {
		return err
	}

	cluster := utils.FindByClusterName(clusters.GetResults(), clusterName)

	if cluster == nil {
		return errors.Errorf("cluster %s not found. You can list all clusters with: qovery cluster list", clusterName)
	}

	_, res, err := service.client.ClustersAPI.DeployCluster(context.Background(), orgId, cluster.Id).Execute()

	if err != nil || res.StatusCode != 200 {
		if res.StatusCode != 200 {
			result, _ := io.ReadAll(res.Body)
			return errors.Errorf("status code: %s ; body: %s ; error: %s", res.Status, string(result), err)
		}
	}

	if watchFlag {
		for {
			status, _, err := service.client.ClustersAPI.GetClusterStatus(context.Background(), orgId, cluster.Id).Execute()
			if err != nil {
				return err
			}

			if utils.IsTerminalClusterState(*status.Status) {
				break
			}

			utils.Println(fmt.Sprintf("Cluster status: %s", utils.GetClusterStatusTextWithColor(status.GetStatus())))

			// sleep here to avoid too many requests
			time.Sleep(5 * time.Second)
		}

		utils.Println(fmt.Sprintf("Cluster %s deployed!", pterm.FgBlue.Sprintf("%s", clusterName)))
	} else {
		utils.Println(fmt.Sprintf("Deploying cluster %s in progress..", pterm.FgBlue.Sprintf("%s", clusterName)))
	}

	return nil
}

func (service *ClusterServiceImpl) StopCluster(organizationName string, clusterName string, watchFlag bool) error {
	orgId, err := usercontext.GetOrganizationContextResourceId(service.client, organizationName)

	if err != nil {
		return err
	}

	clusters, _, err := service.client.ClustersAPI.ListOrganizationCluster(context.Background(), orgId).Execute()

	if err != nil {
		return err
	}

	cluster := utils.FindByClusterName(clusters.GetResults(), clusterName)

	if cluster == nil {
		return fmt.Errorf("cluster %s not found. You can list all clusters with: qovery cluster list", clusterName)
	}

	_, _, err = service.client.ClustersAPI.StopCluster(context.Background(), orgId, cluster.Id).Execute()

	if err != nil {
		return err
	}

	if watchFlag {
		for {
			status, _, err := service.client.ClustersAPI.GetClusterStatus(context.Background(), orgId, cluster.Id).Execute()
			if err != nil {
				return err
			}

			if utils.IsTerminalClusterState(*status.Status) {
				break
			}

			utils.Println(fmt.Sprintf("Cluster status: %s", utils.GetClusterStatusTextWithColor(status.GetStatus())))

			// sleep here to avoid too many requests
			time.Sleep(5 * time.Second)
		}

		utils.Println(fmt.Sprintf("Cluster %s stopped!", pterm.FgBlue.Sprintf("%s", clusterName)))
	} else {
		utils.Println(fmt.Sprintf("Stopping cluster %s in progress..", pterm.FgBlue.Sprintf("%s", clusterName)))
	}

	return nil
}

func (service *ClusterServiceImpl) ListClusters(organizationId string) (*qovery.ClusterResponseList, error) {
	clusters, _, err := service.client.ClustersAPI.ListOrganizationCluster(context.Background(), organizationId).Execute()

	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func (service *ClusterServiceImpl) ListClusterRegions(cloudProviderType qovery.CloudProviderEnum) (*qovery.ClusterRegionResponseList, error) {
	switch cloudProviderType {
	case qovery.CLOUDPROVIDERENUM_GCP:
		regions, _, err := service.client.CloudProviderAPI.ListGcpRegions(context.Background()).Execute()
		if err != nil {
			return nil, err
		}
		return regions, nil
	case qovery.CLOUDPROVIDERENUM_AWS:
		regions, _, err := service.client.CloudProviderAPI.ListAWSRegions(context.Background()).Execute()
		if err != nil {
			return nil, err
		}
		return regions, nil
	case qovery.CLOUDPROVIDERENUM_SCW:
		regions, _, err := service.client.CloudProviderAPI.ListScalewayRegions(context.Background()).Execute()
		if err != nil {
			return nil, err
		}
		return regions, nil
	default:
		return nil, fmt.Errorf("cannot list regions for '%s' cloud provider", cloudProviderType)
	}
}

func (service *ClusterServiceImpl) AskToEditStorageClass(cluster *qovery.Cluster) error {
	storageClassName, err := service.promptUiFactory.RunPrompt("We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage. Enter your storage class name", "")
	if err != nil {
		return err
	}
	if utils.IsEmptyOrBlank(storageClassName) {
		return fmt.Errorf("storage class name should be defined and cannot be empty")
	}

	settings, _, err := service.client.ClustersAPI.GetClusterAdvancedSettings(context.Background(), cluster.Organization.Id, cluster.Id).Execute()
	if err != nil {
		return err
	}

	settings.StorageclassFastSsd = &storageClassName
	_, _, err = service.client.ClustersAPI.EditClusterAdvancedSettings(context.Background(), cluster.Organization.Id, cluster.Id).ClusterAdvancedSettings(*settings).Execute()
	if err != nil {
		return err
	}

	return nil
}
