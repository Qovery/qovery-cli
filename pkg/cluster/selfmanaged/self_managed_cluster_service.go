package selfmanaged

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/cluster/containerregistry"
	"github.com/qovery/qovery-cli/pkg/cluster/credentials"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

type SelfManagedClusterService interface {
	Create(organizationID string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error)
	Configure(cluster *qovery.Cluster) error
	GetInstallationHelmValues(organizationId string, clusterId string) (*string, error)
	GetBaseHelmValuesContent(kubernetesType qovery.CloudProviderEnum) (*string, error)
}

type SelfManagedClusterServiceImpl struct {
	client                          *qovery.APIClient
	clusterService                  cluster.ClusterService
	clusterCredentialsService       credentials.ClusterCredentialsService
	clusterContainerRegistryService containerregistry.ClusterContainerRegistryService
	promptUiFactory                 promptuifactory.PromptUiFactory
}

func NewSelfManagedClusterService(
	client *qovery.APIClient,
	clusterService cluster.ClusterService,
	clusterCredentialsService credentials.ClusterCredentialsService,
	clusterContainerRegistryService containerregistry.ClusterContainerRegistryService,
	promptUiFactory promptuifactory.PromptUiFactory,
) *SelfManagedClusterServiceImpl {
	return &SelfManagedClusterServiceImpl{
		client,
		clusterService,
		clusterCredentialsService,
		clusterContainerRegistryService,
		promptUiFactory,
	}
}

func (service *SelfManagedClusterServiceImpl) Create(
	organizationID string,
	cloudVendor qovery.CloudVendorEnum,
) (*qovery.Cluster, error) {
	cloudProviderType := mapCloudVendorToCloudProviderType(cloudVendor)
	clusterRegion, err := service.findClusterRegion(cloudProviderType)
	if err != nil {
		return nil, err
	}

	credentials, err := service.findOrCreateCredentials(organizationID, cloudProviderType)
	if err != nil {
		return nil, err
	}

	newClusterName, err := service.promptUiFactory.RunPrompt("Give a name to your new cluster", "my-cluster")
	if err != nil {
		return nil, err
	}

	selfManagedMode := qovery.KUBERNETESENUM_SELF_MANAGED
	credentialsId, err := getId(credentials)
	if err != nil {
		return nil, err
	}
	credentialsName, err := getName(credentials)
	if err != nil {
		return nil, err
	}
	cluster, resp, err := service.client.ClustersAPI.CreateCluster(context.Background(), organizationID).ClusterRequest(qovery.ClusterRequest{
		Name:          newClusterName,
		Region:        *clusterRegion,
		CloudProvider: cloudVendor,
		Kubernetes:    &selfManagedMode,
		CloudProviderCredentials: &qovery.ClusterCloudProviderInfoRequest{
			CloudProvider: &cloudProviderType,
			Credentials:   &qovery.ClusterCloudProviderInfoCredentials{Id: &credentialsId, Name: &credentialsName},
			Region:        clusterRegion,
		},
		Features: []qovery.ClusterRequestFeaturesInner{},
	}).Execute()

	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: %v", color.RedString("Error"), string(body))
	}

	return cluster, nil
}

func (service *SelfManagedClusterServiceImpl) Configure(cluster *qovery.Cluster) error {
	// early return for cluster types != On Premise
	if mapCloudVendorToCloudProviderType(cluster.CloudProvider) != qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		return nil
	}

	err := service.clusterContainerRegistryService.AskToEditClusterContainerRegistry(cluster.Organization.Id, cluster.Id)
	if err != nil {
		return err
	}

	err = service.clusterService.AskToEditStorageClass(cluster)
	if err != nil {
		return err
	}

	return nil
}

func (service *SelfManagedClusterServiceImpl) findClusterRegion(
	cloudProviderType qovery.CloudProviderEnum,
) (*string, error) {
	// Early return if we use a ON_PREMISE cluster type
	if cloudProviderType == qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		onPrem := "on-premise"
		return &onPrem, nil
	}

	// Normal path
	clusterRegions, err := service.clusterService.ListClusterRegions(cloudProviderType)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, item := range clusterRegions.Results {
		items = append(items, item.Name)
	}

	utils.Println("Cluster Region:")
	ix, _, err := service.promptUiFactory.RunSelectWithSizeAndSearcher(
		"Select the region where your cluster is installed",
		items,
		30,
		func(input string, index int) bool {
			return strings.Contains(items[index], input)
		},
	)

	if err != nil {
		return nil, err
	}
	return &clusterRegions.Results[ix].Name, nil
}

func (service *SelfManagedClusterServiceImpl) findOrCreateCredentials(
	organizationID string,
	cloudProviderType qovery.CloudProviderEnum,
) (*qovery.ClusterCredentials, error) {
	clusterCreds, err := service.clusterCredentialsService.ListClusterCredentials(organizationID, cloudProviderType)
	if err != nil {
		return nil, err
	}

	var ix = math.MaxInt
	if cloudProviderType == qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		if len(clusterCreds.Results) > 0 {
			ix = 0
		}
	} else {
		var items []string
		for _, creds := range clusterCreds.Results {
			name, err := getName(&creds)
			if err != nil {
				return nil, err
			}
			items = append(items, name)
		}
		items = append(items, "Create new credentials")

		utils.Println("Cluster registry credentials:")
		ixx, _, err := service.promptUiFactory.RunSelectWithSize(
			"Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.",
			items,
			10,
		)
		if err != nil {
			return nil, err
		}
		ix = ixx
	}

	if ix >= len(clusterCreds.Results) {
		return service.clusterCredentialsService.AskToCreateCredentials(organizationID, cloudProviderType)
	}

	return &clusterCreds.Results[ix], nil
}

func (service *SelfManagedClusterServiceImpl) GetInstallationHelmValues(organizationId string, clusterId string) (*string, error) {
	clusterHelmValuesContent, resp, err := service.client.ClustersAPI.GetInstallationHelmValues(
		context.Background(),
		organizationId,
		clusterId,
	).Execute()

	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: %v", color.RedString("Error"), string(body))
	}

	return &clusterHelmValuesContent, nil
}

func getName(creds *qovery.ClusterCredentials) (string, error) {
	switch castedCreds := creds.GetActualInstance().(type) {
	case *qovery.AwsStaticClusterCredentials:
		return castedCreds.GetName(), nil
	case *qovery.AwsRoleClusterCredentials:
		return castedCreds.GetName(), nil
	case *qovery.ScalewayClusterCredentials:
		return castedCreds.GetName(), nil
	case *qovery.GenericClusterCredentials:
		return castedCreds.GetName(), nil
	default:
		return "", errors.New("unknown credentials type")
	}
}

func getId(creds *qovery.ClusterCredentials) (string, error) {
	switch castedCreds := creds.GetActualInstance().(type) {
	case *qovery.AwsStaticClusterCredentials:
		return castedCreds.GetId(), nil
	case *qovery.AwsRoleClusterCredentials:
		return castedCreds.GetId(), nil
	case *qovery.ScalewayClusterCredentials:
		return castedCreds.GetId(), nil
	case *qovery.GenericClusterCredentials:
		return castedCreds.GetId(), nil
	default:
		return "", errors.New("unknown credentials type")
	}
}

func (service *SelfManagedClusterServiceImpl) GetBaseHelmValuesContent(kubernetesType qovery.CloudProviderEnum) (*string, error) {
	// download the appropriate values file
	valuesUrl := ""
	switch kubernetesType {
	case qovery.CLOUDPROVIDERENUM_AWS:
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-aws.yaml"
	case qovery.CLOUDPROVIDERENUM_GCP:
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-gcp.yaml"
	case qovery.CLOUDPROVIDERENUM_SCW:
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-scaleway.yaml"
	case qovery.CLOUDPROVIDERENUM_ON_PREMISE:
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-local.yaml"
	}

	res, err := http.Get(valuesUrl)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	// Check server response
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status while downloading Qovery Helm Values file: %s", res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	s := string(body)
	return &s, nil
}

func mapCloudVendorToCloudProviderType(vendor qovery.CloudVendorEnum) qovery.CloudProviderEnum {
	switch vendor {
	case qovery.CLOUDVENDORENUM_AWS:
		return qovery.CLOUDPROVIDERENUM_AWS
	case qovery.CLOUDVENDORENUM_GCP:
		return qovery.CLOUDPROVIDERENUM_GCP
	case qovery.CLOUDVENDORENUM_SCW:
		return qovery.CLOUDPROVIDERENUM_SCW
	case qovery.CLOUDVENDORENUM_AZURE,
		qovery.CLOUDVENDORENUM_OVH,
		qovery.CLOUDVENDORENUM_DO,
		qovery.CLOUDVENDORENUM_ORACLE,
		qovery.CLOUDVENDORENUM_HETZNER,
		qovery.CLOUDVENDORENUM_IBM,
		qovery.CLOUDVENDORENUM_CIVO,
		qovery.CLOUDVENDORENUM_ON_PREMISE:
		return qovery.CLOUDPROVIDERENUM_ON_PREMISE
	default:
		return qovery.CLOUDPROVIDERENUM_ON_PREMISE
	}
}
