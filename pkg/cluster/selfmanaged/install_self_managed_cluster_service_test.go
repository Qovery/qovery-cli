package selfmanaged

import (
	"errors"
	"github.com/qovery/qovery-client-go"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"testing"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/filewriter"
	"github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldOut := os.Stdout
	defer func() {
		os.Stdout = oldOut
	}()

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	os.Stdout = wOut
	fn()
	_ = wOut.Close()

	outBuf, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}

	return string(outBuf)
}

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
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
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
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
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
		assert.Contains(t, fileWriterService.FileContentWritten, `service.beta.kubernetes.io/azure-load-balancer-internal: "false"`)
		assert.Contains(t, fileWriterService.FileContentWritten, `externalTrafficPolicy: Local`)
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
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
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
		assert.Contains(t, fileWriterService.FileContentWritten, `service.beta.kubernetes.io/azure-load-balancer-internal: "false"`)
		assert.Contains(t, fileWriterService.FileContentWritten, `externalTrafficPolicy: Local`)
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
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
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
		assert.Contains(t, fileWriterService.FileContentWritten, `service.beta.kubernetes.io/azure-load-balancer-internal: "false"`)
		assert.Contains(t, fileWriterService.FileContentWritten, `externalTrafficPolicy: Local`)
	})
}

func TestInjectQoveryClusterGatewayDomain(t *testing.T) {
	t.Run("Should inject qovery-cluster-gateway dns domain from qovery domain when missing", func(t *testing.T) {
		input := `
qovery:
  domain: zf37fd40f.xmx.sh
qovery-cluster-gateway:
  dns: {}
`

		result, err := injectQoveryClusterGatewayDomain(input)

		assert.Nil(t, err)
		assert.Contains(t, *result, "qovery-cluster-gateway:")
		assert.Contains(t, *result, "domain: zf37fd40f.xmx.sh")
	})

	t.Run("Should keep existing qovery-cluster-gateway dns domain", func(t *testing.T) {
		input := `
qovery:
  domain: zf37fd40f.xmx.sh
qovery-cluster-gateway:
  dns:
    domain: custom.example.com
`

		result, err := injectQoveryClusterGatewayDomain(input)

		assert.Nil(t, err)

		var values map[string]interface{}
		err = yaml.Unmarshal([]byte(*result), &values)
		assert.Nil(t, err)

		qoveryClusterGateway := values["qovery-cluster-gateway"].(map[string]interface{})
		dns := qoveryClusterGateway["dns"].(map[string]interface{})
		assert.Equal(t, "custom.example.com", dns["domain"])
	})
}

func TestInjectExternalDNSGatewaySources(t *testing.T) {
	t.Run("Should inject external-dns gateway api sources when missing", func(t *testing.T) {
		input := `
external-dns:
  provider:
    name: pdns
`

		result, err := injectExternalDNSGatewaySources(input)

		assert.Nil(t, err)

		var values map[string]interface{}
		err = yaml.Unmarshal([]byte(*result), &values)
		assert.Nil(t, err)

		externalDNS := values["external-dns"].(map[string]interface{})
		assert.Equal(t, true, externalDNS["enableGatewayListenerSets"])
		assert.Equal(t, []interface{}{"service", "ingress", "gateway-httproute", "gateway-grpcroute"}, externalDNS["sources"])
	})

	t.Run("Should keep existing external-dns sources and gateway listener sets", func(t *testing.T) {
		input := `
external-dns:
  enableGatewayListenerSets: false
  sources:
    - service
    - gateway-httproute
`

		result, err := injectExternalDNSGatewaySources(input)

		assert.Nil(t, err)

		var values map[string]interface{}
		err = yaml.Unmarshal([]byte(*result), &values)
		assert.Nil(t, err)

		externalDNS := values["external-dns"].(map[string]interface{})
		assert.Equal(t, false, externalDNS["enableGatewayListenerSets"])
		assert.Equal(t, []interface{}{"service", "gateway-httproute"}, externalDNS["sources"])
	})
}

func TestInjectEnvoyIngressServices(t *testing.T) {
	t.Run("Should enable envoy ingress services and disable nginx", func(t *testing.T) {
		input := `
services:
  ingress:
    ingress-nginx:
      enabled: true
`

		result, err := injectEnvoyIngressServices(input)

		assert.Nil(t, err)

		var values map[string]interface{}
		err = yaml.Unmarshal([]byte(*result), &values)
		assert.Nil(t, err)

		services := values["services"].(map[string]interface{})
		ingress := services["ingress"].(map[string]interface{})

		assert.Equal(t, false, ingress["ingress-nginx"].(map[string]interface{})["enabled"])
		assert.Equal(t, false, ingress["envoy-gateway-crd"].(map[string]interface{})["enabled"])
		assert.Equal(t, true, ingress["envoy-gateway"].(map[string]interface{})["enabled"])
		assert.Equal(t, true, ingress["qovery-gateway-class"].(map[string]interface{})["enabled"])
		assert.Equal(t, true, ingress["qovery-cluster-gateway"].(map[string]interface{})["enabled"])
	})
}

func TestReuseExistingCluster(t *testing.T) {
	t.Run("Should succeed to reuse an existing self managed cluster", func(t *testing.T) {
		// given
		var testOrganization = organization.CreateTestOrganization()
		var testSelfManagedCluster = CreateSelfManagedTestCluster(testOrganization, qovery.CLOUDVENDORENUM_AWS)
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
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

func TestInstallClusterInjectsQoveryClusterGatewayDomain(t *testing.T) {
	t.Run("Should write values file with qovery-cluster-gateway dns domain for byok installs", func(t *testing.T) {
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
qovery-cluster-gateway:
  dns: {}
`
				return &s, nil
			},
			ResultGetInstallationHelmValues: func() (*string, error) {
				s := `
qovery:
  domain: zf37fd40f.xmx.sh
`
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
					"Select where you want to install Qovery on":                                     "Your AWS EKS cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		_, err := service.InstallCluster()

		assert.Nil(t, err)
		assert.Contains(t, fileWriterService.FileContentWritten, "qovery-cluster-gateway:")
		assert.Contains(t, fileWriterService.FileContentWritten, "domain: zf37fd40f.xmx.sh")
	})
}

func TestInstallClusterInjectsExternalDNSGatewaySources(t *testing.T) {
	t.Run("Should write values file with external-dns gateway api sources for byok installs", func(t *testing.T) {
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
external-dns:
  provider:
    name: pdns
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
					"Select where you want to install Qovery on":                                     "Your Scaleway Kapsule cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		_, err := service.InstallCluster()

		assert.Nil(t, err)
		assert.Contains(t, fileWriterService.FileContentWritten, "external-dns:")
		assert.Contains(t, fileWriterService.FileContentWritten, "enableGatewayListenerSets: true")
		assert.Contains(t, fileWriterService.FileContentWritten, "- service")
		assert.Contains(t, fileWriterService.FileContentWritten, "- ingress")
		assert.Contains(t, fileWriterService.FileContentWritten, "- gateway-httproute")
		assert.Contains(t, fileWriterService.FileContentWritten, "- gateway-grpcroute")
	})
}

func TestInstallClusterEnablesEnvoyIngressServices(t *testing.T) {
	t.Run("Should write values file with envoy ingress services enabled by default", func(t *testing.T) {
		var testOrganization = organization.CreateTestOrganization()
		var organizationService = organization.OrganizationServiceMock{
			ResultAskUserToSelectOrganization: func() (*organization.OrganizationDto, error) {
				return &organization.OrganizationDto{ID: testOrganization.Id, Name: testOrganization.Name}, nil
			},
		}
		var selfManagedService = SelfManagedClusterServiceMock{
			ResultCreate: func(organizationId string, cloudVendor qovery.CloudVendorEnum) (*qovery.Cluster, error) {
				return CreateSelfManagedTestCluster(testOrganization, cloudVendor), nil
			},
			ResultConfigure: func() error {
				return nil
			},
			ResultGetBaseHelmValuesContent: func(kubernetesType qovery.CloudProviderEnum) (*string, error) {
				s := `
services:
  ingress:
    ingress-nginx:
      enabled: true
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
					"Select where you want to install Qovery on":                                     "Your Scaleway Kapsule cluster",
					"Enter your email address to receive expiration notification from Let's Encrypt": "email@test.com",
				},
			),
		)

		_, err := service.InstallCluster()

		assert.Nil(t, err)
		assert.Contains(t, fileWriterService.FileContentWritten, "envoy-gateway-crd:")
		assert.Contains(t, fileWriterService.FileContentWritten, "envoy-gateway:")
		assert.Contains(t, fileWriterService.FileContentWritten, "qovery-gateway-class:")
		assert.Contains(t, fileWriterService.FileContentWritten, "qovery-cluster-gateway:")
		assert.Contains(t, fileWriterService.FileContentWritten, "ingress-nginx:")
		assert.Contains(t, fileWriterService.FileContentWritten, "enabled: false")
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

func TestOutputCommandsToInstallQoveryOnCluster(t *testing.T) {
	output := captureStdout(t, func() {
		outputCommandsToInstallQoveryOnCluster("/tmp/values-test.yaml")
	})

	assert.Contains(t, output, "helm pull qovery/qovery --untar --untardir /tmp/qovery-helm-chart")
	assert.Contains(t, output, "helm template qovery-gateway-crds /tmp/qovery-helm-chart/qovery/charts/gateway-crds-helm")
	assert.Contains(t, output, "kubectl apply --server-side -f -")
	assert.Contains(t, output, "kubectl wait --for=condition=Established --timeout=180s crd/gateways.gateway.networking.k8s.io")
	assert.Contains(t, output, "kubectl wait --for=condition=Established --timeout=180s crd/envoyproxies.gateway.envoyproxy.io")
	assert.Contains(t, output, "--set services.ingress.envoy-gateway-crd.enabled=false")
	assert.Contains(t, output, "--set qovery-cluster-gateway.metrics.enabled=false")
	assert.Contains(t, output, "--set qovery-cluster-gateway.metrics.podMonitor.enabled=false")
	assert.Contains(t, output, "--set services.ingress.envoy-gateway.enabled=false")
	assert.Contains(t, output, "--set services.ingress.qovery-gateway-class.enabled=false")
	assert.Contains(t, output, "--set services.ingress.qovery-cluster-gateway.enabled=false")
}
