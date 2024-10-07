package selfmanaged

import (
	"fmt"
	"github.com/qovery/qovery-client-go"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/filewriter"
	"github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

type InstallSelfManagedClusterService struct {
	organizationService       organization.OrganizationService
	selfManagedClusterService SelfManagedClusterService
	clusterService            cluster.ClusterService
	fileWriterService         filewriter.FileWriterService
	promptUiFactory           promptuifactory.PromptUiFactory
}

func NewInstallSelfManagedClusterService(
	organizationService organization.OrganizationService,
	selfManagedClusterService SelfManagedClusterService,
	clusterService cluster.ClusterService,
	fileWriterService filewriter.FileWriterService,
	promptUiFactory promptuifactory.PromptUiFactory,
) *InstallSelfManagedClusterService {
	return &InstallSelfManagedClusterService{
		organizationService,
		selfManagedClusterService,
		clusterService,
		fileWriterService,
		promptUiFactory,
	}
}

// InstallCluster
// Returns either an error or an indication printed by the caller
func (service *InstallSelfManagedClusterService) InstallCluster() (*string, error) {
	utils.Println("")
	utils.PrintlnInfo(`The following procedure allows you to generate the values files and the helm command necessary to install Qovery on your cluster. You can find more information on our public documentation: https://hub.qovery.com/docs/getting-started/install-qovery/kubernetes/quickstart/`)

	utils.Println("Cluster Type:")
	_, kubernetesType, err := service.promptUiFactory.RunSelectWithSize("Select where you want to install Qovery on",
		[]string{
			"Your AWS EKS cluster",
			"Your GCP GKE cluster",
			"Your Scaleway Kapsule cluster",
			"Your Azure AKS cluster",
			"Your OVH kuke cluster",
			"Your Digital Ocean kube cluster",
			"Your Civo K3S cluster",
			"Your Local Machine",
			"Other",
		},
		10)
	if err != nil {
		return nil, err
	}

	var cloudProviderType qovery.CloudProviderEnum
	if strings.Contains(kubernetesType, "AWS") {
		cloudProviderType = qovery.CLOUDPROVIDERENUM_AWS
	} else if strings.Contains(kubernetesType, "GCP") {
		cloudProviderType = qovery.CLOUDPROVIDERENUM_GCP
	} else if strings.Contains(kubernetesType, "Scaleway") {
		cloudProviderType = qovery.CLOUDPROVIDERENUM_SCW
	} else if strings.Contains(kubernetesType, "Local Machine") {
		indicationMessage := "Please use `qovery demo up` to create a demo cluster on your local machine"
		return &indicationMessage, nil
	} else {
		cloudProviderType = qovery.CLOUDPROVIDERENUM_ON_PREMISE
	}

	organization, err := service.organizationService.AskUserToSelectOrganization()
	if err != nil {
		return nil, err
	}
	if organization == nil {
		return nil, fmt.Errorf("organization not found, please create one on https://console.qovery.com")
	}

	// List cluster and if there is one that already exist for self-managed and this cloud provider
	// propose to re-use it
	clusters, err := service.clusterService.ListClusters(organization.ID)
	if err != nil {
		return nil, err
	}

	var selfManagedClusters []qovery.Cluster
	for _, cluster := range clusters.GetResults() {
		if *cluster.Kubernetes == qovery.KUBERNETESENUM_SELF_MANAGED && cluster.CloudProvider == cloudProviderType {
			selfManagedClusters = append(selfManagedClusters, cluster)
		}
	}

	var cluster *qovery.Cluster
	if len(selfManagedClusters) > 0 {
		// if a self-managed cluster exists, then propose to reuse it or create a new one
		utils.Println("You already have self-managed clusters in your organization.")
		utils.Println("Do you want to reuse one of them or create a new one?")

		_, reuseAClusterPrompt, err := service.promptUiFactory.RunSelect("Reuse or Create a new cluster?", []string{"Reuse a Cluster", "Create a new cluster"})
		if err != nil {
			return nil, err
		}

		if reuseAClusterPrompt == "Reuse a Cluster" {
			utils.Println("Select the cluster you want to reuse:")

			var clusterNameItems []string
			for _, cluster := range selfManagedClusters {
				clusterNameItems = append(clusterNameItems, cluster.Name)
			}

			_, reuseClusterName, err := service.promptUiFactory.RunSelectWithSize("Select the cluster you want to reuse:", clusterNameItems, 10)

			if err != nil {
				return nil, err
			}

			cluster = utils.FindByClusterName(selfManagedClusters, reuseClusterName)
		}
	}

	// We need to create & configure the cluster
	if cluster == nil {
		createdCluster, err := service.selfManagedClusterService.Create(organization.ID, cloudProviderType)
		if err != nil {
			return nil, err
		}
		cluster = createdCluster
		err = service.selfManagedClusterService.Configure(cluster)
		if err != nil {
			return nil, err
		}
	}

	// Email selection for certificate for cert manager
	utils.Println("Contact email for Let's Encrypt certificate:")
	email, err := service.promptUiFactory.RunPrompt("Enter your email address to receive expiration notification from Let's Encrypt", "acme@qovery.com")
	if err != nil {
		return nil, err
	}

	// get the values file for the cluster
	resultClusterHelmValuesContent, err := service.selfManagedClusterService.GetInstallationHelmValues(organization.ID, cluster.Id)
	if err != nil {
		return nil, err
	}
	helmValues := *resultClusterHelmValuesContent

	// inject the email for Cert Manager
	helmValues = strings.ReplaceAll(helmValues, "acme@qovery.com", email)
	helmValues = fmt.Sprintf("%s\n", helmValues)

	// trim lines if they start with "qovery:" or if they contain "set-by-customer"
	qoveryHelmValues, err := service.selfManagedClusterService.GetBaseHelmValuesContent(cloudProviderType)
	if err != nil {
		return nil, err
	}

	helmValues += stripQoverySection(*qoveryHelmValues)

	if strings.Contains(kubernetesType, "Azure") {
		contentWithAKSValues, err := injectAzureAKSValues(helmValues)
		if err != nil {
			return nil, err
		}
		helmValues = *contentWithAKSValues
	}

	// generate the helm values file and output it to the user to ./values-<cluster-name>.yaml
	helmValuesFileName := fmt.Sprintf("values-%s.yaml", strings.ToLower(cluster.Name))

	// get current working directory
	dir, err := os.Getwd()

	if err != nil {
		return nil, err
	}

	helmValuesFileName = filepath.Join(dir, helmValuesFileName)

	helmValuesFileName, err = service.promptUiFactory.RunPrompt("File path to save Helm Values to", helmValuesFileName)

	if err != nil {
		return nil, err
	}

	err = service.fileWriterService.WriteFile(helmValuesFileName, []byte(helmValues), 0644)

	if err != nil {
		return nil, err
	}

	outputCommandsToInstallQoveryOnCluster(helmValuesFileName)

	return nil, nil
}

func stripQoverySection(qoveryHelmValues string) string {
	// Erase the qovery: yaml section to replace it with correct fetched values for this cluster
	// We can't use yaml parser here, because the yaml file contains anchor (&toto *toto) and parsing it will cause those
	// anchors to be replaced with the incorrect values...
	re := regexp.MustCompile("(?m)^qovery:\n( .*\n)+")
	return re.ReplaceAllString(qoveryHelmValues, "")
}

func outputCommandsToInstallQoveryOnCluster(helmValuesFileName string) {
	// give instruction to the user to install the cluster
	utils.Println("")
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.Println("////              Follow these instructions to install your cluster             ////")
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.Println(`
# Add the Qovery Helm repository
helm repo add qovery https://helm.qovery.com`)
	utils.Println("helm repo update")

	utils.Println(fmt.Sprintf(`
# Verify the helm values
Qovery provides you with a default configuration that can be customized based on your needs. More information here: https://hub.qovery.com/docs/getting-started/install-qovery/kubernetes/byok-config
Helm values location: %s
	`, helmValuesFileName))

	utils.Println(fmt.Sprintf(`
# Install Qovery on your cluster first, without some services to avoid circular dependency errors
helm upgrade --install --create-namespace -n qovery -f "%s" --atomic \
	 --set services.certificates.cert-manager-configs.enabled=false \
	 --set services.certificates.qovery-cert-manager-webhook.enabled=false \
	 --set services.qovery.qovery-cluster-agent.enabled=false \
	 --set services.qovery.qovery-engine.enabled=false \
	 qovery qovery/qovery`, helmValuesFileName))

	utils.Println(fmt.Sprintf(`
# Then, re-apply the full Qovery installation with all services
helm upgrade --install --create-namespace -n qovery -f "%s" --wait --atomic qovery qovery/qovery
`, helmValuesFileName))
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.PrintlnInfo("Please note that the installation process may take a few minutes to complete.")
}

func injectAzureAKSValues(clusterHelmValuesContent string) (*string, error) {
	// convert the clusterHelmValuesContent into a YAML object and into a map
	var helmValuesYaml map[string]interface{}

	err := yaml.Unmarshal([]byte(clusterHelmValuesContent), &helmValuesYaml)

	if err != nil {
		return nil, err
	}

	ingressNginx := helmValuesYaml["ingress-nginx"].(map[string]interface{})
	ingressNginxController := ingressNginx["controller"].(map[string]interface{})

	// inject the Azure AKS values
	if ingressNginxController["service"] == nil {
		ingressNginxController["service"] = map[string]interface{}{
			"externalTrafficPolicy": "Local",
			"annotations": map[string]interface{}{
				"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
			},
		}
	} else {
		ingressNginxControllerService := ingressNginxController["service"].(map[string]interface{})
		ingressNginxControllerService["externalTrafficPolicy"] = "Local"

		if ingressNginxControllerService["annotations"] == nil {
			ingressNginxControllerService["annotations"] = map[string]interface{}{
				"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
			}
		} else {
			ingressNginxControllerServiceAnnotations := ingressNginxControllerService["annotations"].(map[string]interface{})
			ingressNginxControllerServiceAnnotations["service.beta.kubernetes.io/azure-load-balancer-internal"] = "true"
		}
	}

	helmValuesYamlBytes, err := yaml.Marshal(helmValuesYaml)

	if err != nil {
		return nil, err
	}
	helmValuesString := string(helmValuesYamlBytes)
	return &helmValuesString, nil
}
