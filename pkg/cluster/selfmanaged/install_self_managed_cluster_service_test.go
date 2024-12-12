package selfmanaged

import (
	"errors"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/filewriter"
	"github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
)

func TestInstallNewCluster(t *testing.T) {
	t.Run("Should return an information message when attempting to create cluster on Local Machine", func(t *testing.T) {
		// given
		var organizationService = organization.OrganizationServiceMock{}
		var selfManagedService = SelfManagedClusterServiceMock{}
		var clusterService = cluster.ClusterServiceMock{}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"Select where you want to install Qovery on": "Your Local Machine",
				},
			),
		)

		// when
		var informationMessage, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
		assert.NotNil(t, informationMessage)
		assert.Equal(t, *informationMessage, "Please use `qovery demo up` to create a demo cluster on your local machine")
	})
	t.Run("Should succeed to create a new self managed cluster", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudProviderType qovery.CloudProviderEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudProviderType), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := "<helm_base_helm_values_content_fetched_from_core"
				return &s, nil
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := "<helm_values_content_fetched_from_core>"
				return &s, nil
			},
		}
		var clusterService = cluster.ClusterServiceMock{
			ResultListClusters: func() (*qovery.ClusterResponseList, error) {
				return &qovery.ClusterResponseList{Results: []qovery.Cluster{}}, nil
			},
		}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{
					"Reuse or Create a new cluster?": true, // should not ask as no cluster exists
				},
				map[string]string{
					"Select where you want to install Qovery on":                                     "Your AWS EKS cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		// when
		var _, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
	})
}
func TestInstallAzureCluster(t *testing.T) {
	t.Run("Should succeed to create a new AKS self managed cluster when ingress-nginx.controller.service is null", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudProviderType qovery.CloudProviderEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudProviderType), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
ingress-nginx:
    controller:
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
				return &s, nil
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := ""
				return &s, nil
			},
		}
		var clusterService = cluster.ClusterServiceMock{
			ResultListClusters: func() (*qovery.ClusterResponseList, error) {
				return &qovery.ClusterResponseList{Results: []qovery.Cluster{}}, nil
			},
		}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"Select where you want to install Qovery on":                                     "Your Azure EKS cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		// when
		var _, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
		var expectedYamlNginxIngress = `
ingress-nginx:
    controller:
        service:
            annotations:
                service.beta.kubernetes.io/azure-load-balancer-internal: "false"
            externalTrafficPolicy: Local
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
		assert.Contains(t, expectedYamlNginxIngress, fileWriterService.FileContentWritten)
	})
	t.Run("Should succeed to create a new AKS self managed cluster when ingress-nginx.controller.service is defined without annotations", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudProviderType qovery.CloudProviderEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudProviderType), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
ingress-nginx:
    controller:
        service:
            externalTrafficPolicy: Local
`
				return &s, nil
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := ""
				return &s, nil
			},
		}
		var clusterService = cluster.ClusterServiceMock{
			ResultListClusters: func() (*qovery.ClusterResponseList, error) {
				return &qovery.ClusterResponseList{Results: []qovery.Cluster{}}, nil
			},
		}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"Select where you want to install Qovery on":                                     "Your Azure EKS cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		// when
		var _, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
		var expectedYamlNginxIngress = `
ingress-nginx:
    controller:
        service:
            annotations:
                service.beta.kubernetes.io/azure-load-balancer-internal: "false"
            externalTrafficPolicy: Local
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
		assert.Contains(t, expectedYamlNginxIngress, fileWriterService.FileContentWritten)
	})
	t.Run("Should succeed to create a new AKS self managed cluster when ingress-nginx.controller.service is defined with annotations", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudProviderType qovery.CloudProviderEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudProviderType), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
ingress-nginx:
    controller:
        service:
            annotations:
            externalTrafficPolicy: Local
`
				return &s, nil
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := ""
				return &s, nil
			},
		}
		var clusterService = cluster.ClusterServiceMock{
			ResultListClusters: func() (*qovery.ClusterResponseList, error) {
				return &qovery.ClusterResponseList{Results: []qovery.Cluster{}}, nil
			},
		}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"Select where you want to install Qovery on":                                     "Your Azure EKS cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		// when
		var _, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
		var expectedYamlNginxIngress = `
ingress-nginx:
    controller:
        service:
            annotations:
                service.beta.kubernetes.io/azure-load-balancer-internal: "false"
            externalTrafficPolicy: Local
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
		assert.Contains(t, expectedYamlNginxIngress, fileWriterService.FileContentWritten)
	})
}
func TestReuseExistingCluster(t *testing.T) {
	t.Run("Should succeed to reuse an existing self managed cluster", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var testSelfManagedCluster = CreateSelfManagedTestCluster(testOrganization, qovery.CLOUDPROVIDERENUM_AWS)
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudProviderType qovery.CloudProviderEnum) (*qovery.Cluster, error) {
				return nil, errors.New("should not create self managed cluster")
			},
			ResultConfigure: func() error {
				return errors.New("should not configure an existing self managed cluster")
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := "<helm_values_content_fetched_from_core>"
				return &s, nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := "<helm_base_helm_values_content_fetched_from_core"
				return &s, nil
			},
		}
		var clusterService = cluster.ClusterServiceMock{
			ResultListClusters: func() (*qovery.ClusterResponseList, error) {
				return &qovery.ClusterResponseList{Results: []qovery.Cluster{*testSelfManagedCluster}}, nil
			},
		}
		var fileWriterService = filewriter.FileWriterServiceMock{}
		var service = NewInstallSelfManagedClusterService(
			&organizationService,
			&selfManagedService,
			&clusterService,
			&fileWriterService,
			promptuifactory.NewPromptUiFactoryMock(
				map[string]bool{},
				map[string]string{
					"Select where you want to install Qovery on":                                     "Your AWS EKS cluster",
					"Reuse or Create a new cluster?":                                                 "Reuse a Cluster",
					"Select the cluster you want to reuse:":                                          testSelfManagedCluster.Name,
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		// when
		var _, err = service.InstallCluster()

		// then
		assert.Nil(t, err)
	})
}

func TestStripQoverySection(t *testing.T) {
	helmValues := `
services:
  qovery:
    qovery-cluster-agent:
      enabled: true
    qovery-shell-agent:
      enabled: true
    qovery-engine:
      enabled: true
    qovery-priority-class:
      enabled: true
  ingress:
    ingress-nginx:
      enabled: true
  dns:
    external-dns:
      enabled: true
  logging:
    loki:
      enabled: true
    promtail:
      enabled: true
  certificates:
    cert-manager:
      enabled: true
    cert-manager-configs:
      enabled: true
    qovery-cert-manager-webhook:
      enabled: true
  observability:
    metrics-server:
      enabled: true
  aws:
    q-storageclass-aws:
      enabled: true
    aws-ebs-csi-driver:
      enabled: false
    aws-load-balancer-controller:
      enabled: false
  gcp:
    q-storageclass-gcp:
      enabled: false
  scaleway:
    q-storageclass-scaleway:
      enabled: false
qovery:
  clusterId: &clusterId set-by-customer
  clusterShortId: &clusterShortId set-by-customer
  organizationId: &organizationId set-by-customer
  jwtToken: &jwtToken set-by-customer
  rootDomain: &rootDomain set-by-customer
  domain: &domain set-by-customer
  domainWildcard: &domainWildcard set-by-customer
  qoveryDnsUrl: &qoveryDnsUrl set-by-customer
  agentGatewayUrl: &agentGatewayUrl set-by-customer
  engineGatewayUrl: &engineGatewayUrl set-by-customer
  lokiUrl: &lokiUrl set-by-customer
  promtailLokiUrl: &promtailLokiUrl set-by-customer
  acmeEmailAddr: &acmeEmailAddr set-by-customer
  externalDnsPrefix: &externalDnsPrefix set-by-customer
  architectures: &architectures set-by-customer
  engineVersion: &engineVersion set-by-customer
  shellAgentVersion: &shellAgentVersion set-by-customer
  clusterAgentVersion: &clusterAgentVersion set-by-customer
qovery-cluster-agent:
  fullnameOverride: qovery-shell-agent
  image:
    tag: *clusterAgentVersion
  environmentVariables:
    CLUSTER_ID: *clusterId
    CLUSTER_JWT_TOKEN: *jwtToken
    GRPC_SERVER: *agentGatewayUrl
    LOKI_URL: *lokiUrl
    ORGANIZATION_ID: *organizationId
  useSelfSignCertificate: true
`
	resultHelmValues := `
services:
  qovery:
    qovery-cluster-agent:
      enabled: true
    qovery-shell-agent:
      enabled: true
    qovery-engine:
      enabled: true
    qovery-priority-class:
      enabled: true
  ingress:
    ingress-nginx:
      enabled: true
  dns:
    external-dns:
      enabled: true
  logging:
    loki:
      enabled: true
    promtail:
      enabled: true
  certificates:
    cert-manager:
      enabled: true
    cert-manager-configs:
      enabled: true
    qovery-cert-manager-webhook:
      enabled: true
  observability:
    metrics-server:
      enabled: true
  aws:
    q-storageclass-aws:
      enabled: true
    aws-ebs-csi-driver:
      enabled: false
    aws-load-balancer-controller:
      enabled: false
  gcp:
    q-storageclass-gcp:
      enabled: false
  scaleway:
    q-storageclass-scaleway:
      enabled: false
qovery-cluster-agent:
  fullnameOverride: qovery-shell-agent
  image:
    tag: *clusterAgentVersion
  environmentVariables:
    CLUSTER_ID: *clusterId
    CLUSTER_JWT_TOKEN: *jwtToken
    GRPC_SERVER: *agentGatewayUrl
    LOKI_URL: *lokiUrl
    ORGANIZATION_ID: *organizationId
  useSelfSignCertificate: true
`
	t.Run("Should strip qovery section for yanl file", func(t *testing.T) {
		ret := stripQoverySection(helmValues)
		assert.Equal(t, resultHelmValues, ret)
	})
}
