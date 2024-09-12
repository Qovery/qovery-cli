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
                service.beta.kubernetes.io/azure-load-balancer-internal: "true"
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
                service.beta.kubernetes.io/azure-load-balancer-internal: "true"
            externalTrafficPolicy: Local
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
		/*
			"\ningress-nginx:\n    controller:\n        service:\n            externalTrafficPolicy: Local\n            annotations:\n                service.beta.kubernetes.io/azure-load-balancer-internal: \"true\"\n        useComponentLabel: true\n    fullnameOverride: ingress-nginx\n"
			"\ningress-nginx:\n    controller:\n        service:\n            annotations:\n                service.beta.kubernetes.io/azure-load-balancer-internal: \"true\"\n            externalTrafficPolicy: Local\n"
		*/
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
                service.beta.kubernetes.io/azure-load-balancer-internal: "true"
            externalTrafficPolicy: Local
        useComponentLabel: true
    fullnameOverride: ingress-nginx
`
		/*
			"\ningress-nginx:\n    controller:\n        service:\n            annotations:\n                custom-annotation: \"value\"\n                service.beta.kubernetes.io/azure-load-balancer-internal: \"true\"\n            externalTrafficPolicy: Local\n        useComponentLabel: true\n    fullnameOverride: ingress-nginx\n"
			"\ningress-nginx:\n    controller:\n        service:\n            annotations:\n                custom-annotation: value\n                service.beta.kubernetes.io/azure-load-balancer-internal: \"true\"\n            enabled: true\n            externalTrafficPolicy: Local\n"
		*/
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
