package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var clusterInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Qovery on your cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		// clusterTypePrompt for cluster type
		// select between Managed By Qovery or Self Managed or Local Machine
		// if Managed By Qovery, quit and print message to use the web interface console.qovery.com
		// if Local Machine, quit and print message to use the `qovery demo up` on the local machine
		utils.Println("Cluster Type:")
		clusterTypePrompt := promptui.Select{
			Label: "Select where you want to install Qovery on:",
			Items: []string{"Your Kubernetes Cluster", "Your Local Machine"},
		}

		_, kubernetesType, err := clusterTypePrompt.Run()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if kubernetesType == "Local Machine" {
			utils.PrintlnInfo("Please use `qovery demo up` to create a demo cluster on your local machine")
			os.Exit(0)
		}

		// if Self Managed, continue with the installation process

		organization, err := utils.SelectOrganization()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if organization == nil {
			utils.PrintlnError(fmt.Errorf("organizations not found, please create one on https://console.qovery.com"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// check that the cluster name is unique
		clusters, _, err := client.ClustersAPI.ListOrganizationCluster(context.Background(), string(organization.ID)).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var selfManagedClusters []qovery.Cluster
		for _, cluster := range clusters.GetResults() {
			if cluster.CloudProvider == qovery.CLOUDPROVIDERENUM_ON_PREMISE {
				selfManagedClusters = append(selfManagedClusters, cluster)
			}
		}

		var cluster *qovery.Cluster
		if len(selfManagedClusters) > 0 {
			// if a self-managed cluster exist, then propose to reuse it or create a new one
			utils.Println("You already have self-managed clusters in your organization.")
			utils.Println("Do you want to reuse one of them or create a new one?")
			reuseOrCreateNewClusterPrompt := promptui.Select{
				Label: "Reuse or Create a new cluster?",
				Items: []string{"Reuse a Cluster", "Create a new cluster"},
			}

			_, reuseOrCreateNewCluster, err := reuseOrCreateNewClusterPrompt.Run()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			if reuseOrCreateNewCluster == "Reuse a Cluster" {
				utils.Println("Select the cluster you want to reuse:")

				var clusterNameItems []string

				for _, cluster := range selfManagedClusters {
					clusterNameItems = append(clusterNameItems, cluster.Name)
				}

				reuseClusterPrompt := promptui.Select{
					Label: "Select the cluster you want to reuse",
					Items: clusterNameItems,
				}

				_, reuseClusterName, err := reuseClusterPrompt.Run()

				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
					panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
				}

				cluster = utils.FindByClusterName(selfManagedClusters, reuseClusterName)
			}
		}

		// clusterTypePrompt where the cluster is located (AWS, GCP, Azure, Scaleway, OVH Cloud, Digital Ocean, Civo, Other, etc.)
		utils.Println("Kubernetes Type:")
		kubernetesTypePrompt := promptui.Select{
			Label: "Select your Kubernetes type",
			Items: []string{
				"AWS EKS",
				"GCP GKE",
				"Azure AKS",
				"Scaleway Kapsule",
				"OVH Cloud Kubernetes",
				"Digital Ocean Kubernetes",
				"Civo K3S",
				"On Premise",
				"Other",
			},
		}

		_, kubernetesType, err = kubernetesTypePrompt.Run()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		kubernetesTypeOther := ""
		if kubernetesType == "Other" {
			utils.Println("Other: where your Kubernetes cluster is located?")
			clusterLocationOtherPrompt := promptui.Prompt{
				Label: "Enter the location of your Kubernetes cluster (optional)",
			}

			kubernetesType, err = clusterLocationOtherPrompt.Run()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			kubernetesTypeOther = kubernetesType
		}

		// TODO clusterTypePrompt for the Kubernetes version -- propose a list of versions
		// TODO based on the version, display a message explaining if Qovery supports the version or not

		if cluster == nil {
			// clusterTypePrompt for cluster name
			mClusterName := promptForClusterName(fmt.Sprintf("my-cluster-%s", utils.RandStringBytes(4)))

			for {
				cluster := utils.FindByClusterName(clusters.GetResults(), mClusterName)
				if cluster == nil {
					break
				}

				utils.PrintlnError(fmt.Errorf("cluster %s already exists", mClusterName))
				utils.Println("Here are the clusters that already exist in your organization:")

				for _, cluster := range clusters.GetResults() {
					utils.Println(fmt.Sprintf("- %s", cluster.Name))
				}

				utils.Println("\nPlease choose another name that is not already in use.\n")

				mClusterName = promptForClusterName(mClusterName)
			}

			// API call to get or create the on-premise account
			onPremiseAccount, err := getOrCreateOnPremiseAccount(utils.GetAuthorizationHeaderValue(tokenType, token), string(organization.ID))
			if err != nil {

				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			// API call to create the self-managed cluster and link it to the on-premise account
			description := fmt.Sprintf("Cluster running on %s (%s)", kubernetesType, kubernetesTypeOther)

			k := qovery.KUBERNETESENUM_SELF_MANAGED
			cp := qovery.CLOUDPROVIDERENUM_ON_PREMISE
			region := "on-premise"

			infoCredentialsName := "on-premise"
			infoCredentials := qovery.ClusterCloudProviderInfoCredentials{
				Id:   &onPremiseAccount,
				Name: &infoCredentialsName,
			}

			cloudProviderCredentials := qovery.ClusterCloudProviderInfoRequest{
				CloudProvider: &cp,
				Credentials:   &infoCredentials,
				Region:        &region,
			}

			cluster, _, err = client.ClustersAPI.CreateCluster(
				context.Background(),
				string(organization.ID),
			).ClusterRequest(qovery.ClusterRequest{
				Name:                     mClusterName,
				Description:              &description,
				Region:                   region,
				CloudProvider:            cp,
				Kubernetes:               &k,
				Production:               utils.Bool(false),
				Features:                 []qovery.ClusterRequestFeaturesInner{},
				CloudProviderCredentials: &cloudProviderCredentials,
			}).Execute()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		// propose to configure the container registry (optional);
		// by default it is a local registry on the cluster (not recommended for production)
		// configure container registry (optional)
		utils.Println("")
		utils.Println(`Qovery must uses a container registry to mirror your images. 
You can use the default registry (local) on your cluster or a managed registry.
We recommend using a managed registry for intensive deployments.
This can be configured later in the Qovery Console.`)

		configureContainerRegistryPrompt := promptui.Select{
			Label: "Do you want to configure a container registry?",
			Items: []string{"Yes", "No"},
		}

		_, configureContainerRegistry, err := configureContainerRegistryPrompt.Run()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if configureContainerRegistry == "Yes" {
			showContainerRegistryConfiguration(cluster, organization, kubernetesType)
		}

		// get the email of the user for Cert Manager
		utils.Println("Email for Cert Manager / Let's Encrypt:")
		emailPrompt := promptui.Prompt{
			Label:   "Enter your email address for Cert Manager",
			Default: "acme@qovery.com",
		}

		email, err := emailPrompt.Run()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// get the values file for the cluster
		clusterHelmValuesContent, _, err := client.ClustersAPI.GetInstallationHelmValues(
			context.Background(),
			string(organization.ID),
			cluster.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// inject the email for Cert Manager
		clusterHelmValuesContent = strings.ReplaceAll(clusterHelmValuesContent, "acme@qovery.com", email)

		finalClusterHelmValuesContent := fmt.Sprintf("%s\n", clusterHelmValuesContent)

		// trim lines if they start with "qovery:" or if they contain "set-by-customer"
		for _, line := range strings.Split(getBaseHelmValuesContent(kubernetesType), "\n") {
			if strings.HasPrefix(line, "qovery:") || strings.Contains(line, "set-by-customer") {
				continue
			}
			finalClusterHelmValuesContent += line + "\n"
		}

		if kubernetesType == "Azure AKS" {
			finalClusterHelmValuesContent = injectAzureAKSValues(finalClusterHelmValuesContent)
		}

		// generate the helm values file and output it to the user to ./values-<cluster-name>.yaml
		helmValuesFileName := fmt.Sprintf("values-%s.yaml", strings.ToLower(cluster.Name))

		// get current working directory
		dir, err := os.Getwd()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helmValuesFileName = filepath.Join(dir, helmValuesFileName)

		utils.Println("Save Helm Values to a file:")
		helmValuesPathPrompt := promptui.Prompt{
			Label:   "File path to save Helm Values to",
			Default: helmValuesFileName,
		}

		helmValuesFileName, err = helmValuesPathPrompt.Run()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = os.WriteFile(helmValuesFileName, []byte(finalClusterHelmValuesContent), 0644)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		outputCommandsToInstallQoveryOnCluster(helmValuesFileName)
	},
}

func showContainerRegistryConfiguration(cluster *qovery.Cluster, organization *utils.Organization, kubernetesType string) {
	utils.Println("\nPlease configure the container registry in the Qovery Console:")
	utils.Println(fmt.Sprintf("https://console.qovery.com/organization/%s/settings/container-registries", string(organization.ID)))
	utils.Println(fmt.Sprintf("The registry name is: registry-%s", cluster.Id))
	utils.Println("")

	if kubernetesType == "Azure AKS" {
		utils.Println("For Azure AKS, you can:")
		utils.Println("- Create a container registry in Azure Container Registry")
		utils.Println("- Turn on the Admin User in the Azure Container Registry (Access Keys section)")
		utils.Println("- Use the GENERIC_CR as the container registry in Qovery")
		utils.Println("- Your Azure Container Registry URL is: https://<registry-name>.azurecr.io/v2/")
		utils.Println("- Your Azure Container Registry Username is: <access key username>")
		utils.Println("- Your Azure Container Registry Password is: <access key password>")
		utils.Println("Note: you can also use another container registry if you prefer.")
	}

	if kubernetesType == "AWS EKS" {
		utils.Println("For AWS EKS, you can:")
		utils.Println("- Create a container registry in Amazon Elastic Container Registry (ECR)")
		utils.Println("- Use the ECR as the container registry in Qovery")
		utils.Println("- Set your credentials")
		utils.Println("Note: you can also use another container registry if you prefer.")
	}

	//if kubernetesType == "GCP GKE" {
	// TODO implement GCP GKE container registry configuration
	//}

	if kubernetesType == "Scaleway Kapsule" {
		utils.Println("For Scaleway Kapsule, you can:")
		utils.Println("- Create a container registry in Scaleway Container Registry")
		utils.Println("- Use the Scaleway Container Registry as the container registry in Qovery")
		utils.Println("- Set your credentials")
		utils.Println("Note: you can also use another container registry if you prefer.")
	}

	// if kubernetesType == "OVH Cloud Kubernetes" {
	// TODO implement OVH Cloud Kubernetes container registry configuration
	// }

	if kubernetesType == "Digital Ocean Kubernetes" {
		utils.Println("For Digital Ocean Kubernetes, you can:")
		utils.Println("- Create a container registry in Digital Ocean Container Registry")
		utils.Println("- Use the Digital Ocean Container Registry as the container registry in Qovery")
		utils.Println("- Set your credentials")
		utils.Println("Note: you can also use another container registry if you prefer.")
	}

	//if kubernetesType == "Civo K3S" {
	// TODO implement Civo K3S container registry configuration
	//}

	if kubernetesType == "On Premise" {
		utils.Println("For On Premise, you can connect any container registry you want.")
	}

	utils.Println("")
}

func outputCommandsToInstallQoveryOnCluster(helmValuesFileName string) {
	// give instruction to the user to install the cluster
	utils.Println("")
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.Println("//// Please copy/paste the following commands to install Qovery on your cluster ////")
	utils.Println("////          ⚠️ Check the values file before running the commands ⚠️           ////")
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.Println(`
# Add the Qovery Helm repository
helm repo add qovery https://helm.qovery.com`)
	utils.Println("helm repo update")

	utils.Println(fmt.Sprintf(`
# Install Qovery on your cluster first, without some some services to avoid circular dependencies errors
helm upgrade --install --create-namespace -n qovery -f "%s" --atomic \
	 --set services.certificates.cert-manager-configs.enabled=false \
	 --set services.certificates.qovery-cert-manager-webhook.enabled=false \
	 --set services.qovery.qovery-cluster-agent.enabled=false \
	 --set services.qovery.qovery-engine.enabled=false \
	 qovery qovery/qovery`, helmValuesFileName))

	utils.Println(fmt.Sprintf(`
# Then, re-apply the full Qovery installation with all services
helm upgrade --install --create-namespace -n qovery -f \"%s\" --wait --atomic qovery qovery/qovery
`, helmValuesFileName))
	utils.Println("////////////////////////////////////////////////////////////////////////////////////")
	utils.PrintlnInfo("Please note that the installation process may take a few minutes to complete.")
}

func promptForClusterName(defaultName string) string {
	utils.Println("Cluster Name:")
	clusterNamePrompt := promptui.Prompt{
		Label:   "Your Cluster Name",
		Default: defaultName,
	}

	mClusterName, err := clusterNamePrompt.Run()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return mClusterName
}

func injectAzureAKSValues(clusterHelmValuesContent string) string {
	// convert the clusterHelmValuesContent into a YAML object and into a map
	var helmValuesYaml map[string]interface{}

	err := yaml.Unmarshal([]byte(clusterHelmValuesContent), &helmValuesYaml)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
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
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	return string(helmValuesYamlBytes)
}

type onPremiseCredentials struct {
	ID string `json:"id"`
}

type onPremiseResults struct {
	Results []onPremiseCredentials `json:"results"`
}

func getOrCreateOnPremiseAccount(authorizationToken string, organizationID string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.qovery.com/organization/"+organizationID+"/onPremise/credentials", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", authorizationToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var results onPremiseResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		return "", err
	}

	if len(results.Results) > 0 {
		return results.Results[0].ID, nil
	}

	req, err = http.NewRequest("POST", "https://api.qovery.com/organization/"+organizationID+"/onPremise/credentials", bytes.NewBuffer([]byte(`{"name": "on-premise"}`)))
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", authorizationToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var credentials onPremiseCredentials
	err = json.Unmarshal(body, &credentials)
	if err != nil {
		return "", err
	}

	return credentials.ID, nil
}

func getBaseHelmValuesContent(kubernetesType string) string {
	// download the appropriate values file
	// default: https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-local.yaml
	valuesUrl := "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-local.yaml"

	switch kubernetesType {
	case "AWS EKS":
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-aws.yaml"
	case "GCP GKE":
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-gcp.yaml"
	case "Scaleway Kapsule":
		valuesUrl = "https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-scaleway.yaml"
	}

	res, err := http.Get(valuesUrl)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	defer res.Body.Close()

	// Check server response
	if res.StatusCode != http.StatusOK {
		utils.PrintlnError(fmt.Errorf("bad status while downloading Qovery Helm Values file: %s", res.Status))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(body)
}

func init() {
	clusterCmd.AddCommand(clusterInstallCmd)
}
