package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		}

		client := utils.GetQoveryClient(tokenType, token)

		utils.Println("")
		utils.PrintlnInfo(`The following procedure allows you to generate the values files and the helm command necessary to install Qovery on your cluster. You can find more information on our public documentation: https://hub.qovery.com/docs/getting-started/install-qovery/kubernetes/quickstart/
		`)

		// clusterTypePrompt for cluster type
		// select between Managed By Qovery or Self Managed or Local Machine
		// if Managed By Qovery, quit and print message to use the web interface console.qovery.com
		// if Local Machine, quit and print message to use the `qovery demo up` on the local machine
		utils.Println("Cluster Type:")
		clusterTypePrompt := promptui.Select{
			Label: "Select where you want to install Qovery on",
			Items: []string{
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
			Size: 10,
		}

		_, kubernetesType, err := clusterTypePrompt.Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		cloudProviderType := qovery.CLOUDPROVIDERENUM_AWS
		if strings.Contains(kubernetesType, "AWS") {
			cloudProviderType = qovery.CLOUDPROVIDERENUM_AWS
		} else if strings.Contains(kubernetesType, "GCP") {
			cloudProviderType = qovery.CLOUDPROVIDERENUM_GCP
		} else if strings.Contains(kubernetesType, "Scaleway") {
			cloudProviderType = qovery.CLOUDPROVIDERENUM_SCW
		} else if strings.Contains(kubernetesType, "Local Machine") {
			utils.PrintlnInfo("Please use `qovery demo up` to create a demo cluster on your local machine")
			os.Exit(0)
		} else {
			cloudProviderType = qovery.CLOUDPROVIDERENUM_ON_PREMISE
		}

		// Select the correct organization
		organization, err := utils.SelectOrganization()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		if organization == nil {
			utils.PrintlnError(fmt.Errorf("organizations not found, please create one on https://console.qovery.com"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// List cluster and if there is one that already exist for self-managed and this cloud provider
		// propose to re-use it
		clusters, _, err := client.ClustersAPI.ListOrganizationCluster(context.Background(), string(organization.ID)).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		var selfManagedClusters []qovery.Cluster
		for _, cluster := range clusters.GetResults() {
			if *cluster.Kubernetes == qovery.KUBERNETESENUM_SELF_MANAGED && cluster.CloudProvider == cloudProviderType {
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

			ix, _, err := reuseOrCreateNewClusterPrompt.Run()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			if ix == 0 {
				utils.Println("Select the cluster you want to reuse:")

				var clusterNameItems []string
				for _, cluster := range selfManagedClusters {
					clusterNameItems = append(clusterNameItems, cluster.Name)
				}
				reuseClusterPrompt := promptui.Select{
					Label: "Select the cluster you want to reuse",
					Items: clusterNameItems,
					Size:  10,
				}

				_, reuseClusterName, err := reuseClusterPrompt.Run()

				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}

				cluster = utils.FindByClusterName(selfManagedClusters, reuseClusterName)
			}
		}

		// We need to create the cluster
		if cluster == nil {
			var clusterCreds *qovery.ClusterCredentialsResponseList
			var clusterRegions *qovery.ClusterRegionResponseList
			switch cloudProviderType {
			case qovery.CLOUDPROVIDERENUM_GCP:
				regions, _, err := client.CloudProviderAPI.ListGcpRegions(context.Background()).Execute()
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterRegions = regions

				req := client.CloudProviderCredentialsAPI.ListGcpCredentials(context.Background(), string(organization.ID))
				creds, _, err := client.CloudProviderCredentialsAPI.ListGcpCredentialsExecute(req)
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterCreds = creds
			case qovery.CLOUDPROVIDERENUM_AWS:
				regions, _, err := client.CloudProviderAPI.ListAWSRegions(context.Background()).Execute()
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterRegions = regions

				req := client.CloudProviderCredentialsAPI.ListAWSCredentials(context.Background(), string(organization.ID))
				creds, _, err := client.CloudProviderCredentialsAPI.ListAWSCredentialsExecute(req)
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterCreds = creds
			case qovery.CLOUDPROVIDERENUM_SCW:
				regions, _, err := client.CloudProviderAPI.ListScalewayRegions(context.Background()).Execute()
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterRegions = regions

				req := client.CloudProviderCredentialsAPI.ListScalewayCredentials(context.Background(), string(organization.ID))
				creds, _, err := client.CloudProviderCredentialsAPI.ListScalewayCredentialsExecute(req)
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterCreds = creds

			case qovery.CLOUDPROVIDERENUM_ON_PREMISE:
				req := client.CloudProviderCredentialsAPI.ListOnPremiseCredentials(context.Background(), string(organization.ID))
				creds, _, err := client.CloudProviderCredentialsAPI.ListOnPremiseCredentialsExecute(req)
				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				clusterCreds = creds
			}

			// Select the region
			clusterRegion := func() *string {
				if clusterRegions == nil {
					onPrem := "on-premise"
					return &onPrem
				}

				var items []string
				for _, item := range clusterRegions.Results {
					items = append(items, item.Name)
				}

				utils.Println("Cluster Region:")
				prompt := promptui.Select{
					Label: "Select the region where your cluster is installed",
					Items: items,
					Size:  30,
					Searcher: func(input string, index int) bool {
						return strings.Contains(items[index], input)
					},
					StartInSearchMode: true,
				}
				ix, _, err := prompt.Run()

				if err != nil {
					utils.PrintlnError(err)
					os.Exit(1)
				}
				return &clusterRegions.Results[ix].Name
			}()

			// Select the credentials to use
			credentials := func() qovery.ClusterCredentials {
				var ix = math.MaxInt

				if cloudProviderType == qovery.CLOUDPROVIDERENUM_ON_PREMISE {
					if len(clusterCreds.Results) > 0 {
						ix = 0
					}
				} else {
					var items []string
					for _, creds := range clusterCreds.Results {
						items = append(items, creds.Name)
					}
					items = append(items, "Create new credentials")

					utils.Println("Cluster registry credentials:")
					prompt := promptui.Select{
						Label: "Which credentials do you want to use for the container registry ? A container registry is necessary to build and mirror the images deployed on your cluster.",
						Items: items,
						Size:  10,
					}
					ixx, _, err := prompt.Run()
					if err != nil {
						utils.PrintlnError(err)
						os.Exit(1)
					}
					ix = ixx
				}

				if ix >= len(clusterCreds.Results) {
					return *createCredentials(client, string(organization.ID), cloudProviderType)
				}

				return clusterCreds.Results[ix]
			}()

			selfManagedMode := qovery.KUBERNETESENUM_SELF_MANAGED
			clusterRes, resp, err := client.ClustersAPI.CreateCluster(context.Background(), string(organization.ID)).ClusterRequest(qovery.ClusterRequest{
				Name:          promptForClusterName("my-cluster"),
				Region:        *clusterRegion,
				CloudProvider: cloudProviderType,
				Kubernetes:    &selfManagedMode,
				CloudProviderCredentials: &qovery.ClusterCloudProviderInfoRequest{
					CloudProvider: &cloudProviderType,
					Credentials:   &qovery.ClusterCloudProviderInfoCredentials{Id: &credentials.Id, Name: &credentials.Name},
					Region:        clusterRegion,
				},
				Features: []qovery.ClusterRequestFeaturesInner{},
			}).Execute()

			if err != nil {
				utils.PrintlnError(err)
				body, _ := io.ReadAll(resp.Body)
				fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
				os.Exit(1)
			}
			cluster = clusterRes
		}

		configureRegistry(client, cluster)
		configureStorageClass(client, cluster)

		// Email selection for certificate
		email := func() string {
			// get the email of the user for Cert Manager
			utils.Println("Contact email for Let's Encrypt certificate:")
			emailPrompt := promptui.Prompt{
				Label:   "Enter your email address to receive expiration notification from Let's Encrypt",
				Default: "acme@qovery.com",
			}

			email, err := emailPrompt.Run()

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}
			return email
		}()

		// get the values file for the cluster
		clusterHelmValuesContent, _, err := client.ClustersAPI.GetInstallationHelmValues(
			context.Background(),
			string(organization.ID),
			cluster.Id,
		).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		// inject the email for Cert Manager
		clusterHelmValuesContent = strings.ReplaceAll(clusterHelmValuesContent, "acme@qovery.com", email)

		finalClusterHelmValuesContent := fmt.Sprintf("%s\n", clusterHelmValuesContent)

		// trim lines if they start with "qovery:" or if they contain "set-by-customer"
		for _, line := range strings.Split(getBaseHelmValuesContent(cloudProviderType), "\n") {
			if strings.HasPrefix(line, "qovery:") || strings.Contains(line, "set-by-customer") {
				continue
			}
			finalClusterHelmValuesContent += line + "\n"
		}

		if strings.Contains(kubernetesType, "Azure") {
			finalClusterHelmValuesContent = injectAzureAKSValues(finalClusterHelmValuesContent)
		}

		// generate the helm values file and output it to the user to ./values-<cluster-name>.yaml
		helmValuesFileName := fmt.Sprintf("values-%s.yaml", strings.ToLower(cluster.Name))

		// get current working directory
		dir, err := os.Getwd()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
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
		}

		err = os.WriteFile(helmValuesFileName, []byte(finalClusterHelmValuesContent), 0644)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		outputCommandsToInstallQoveryOnCluster(helmValuesFileName)

		utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
	},
}

func createCredentials(client *qovery.APIClient, orgaId string, providerType qovery.CloudProviderEnum) *qovery.ClusterCredentials {
	credsName, err := func() *promptui.Prompt {
		return &promptui.Prompt{
			Label:   "Give a name to your credentials",
			Default: "",
		}
	}().Run()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	switch providerType {
	case qovery.CLOUDPROVIDERENUM_AWS:
		accessKey, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your AWS access key",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		secretKey, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your AWS secret key",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		creds, resp, err := client.CloudProviderCredentialsAPI.CreateAWSCredentials(context.Background(), orgaId).AwsCredentialsRequest(qovery.AwsCredentialsRequest{
			Name:            credsName,
			AccessKeyId:     accessKey,
			SecretAccessKey: secretKey,
		}).Execute()
		if err != nil {
			utils.PrintlnError(err)
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
			os.Exit(1)
		}
		return creds

	case qovery.CLOUDPROVIDERENUM_SCW:
		accessKey, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your SCW access key",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		secretKey, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your SCW secret key",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		organizationId, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your SCW organization ID",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		projectId, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your SCW project ID",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		creds, resp, err := client.CloudProviderCredentialsAPI.CreateScalewayCredentials(context.Background(), orgaId).ScalewayCredentialsRequest(qovery.ScalewayCredentialsRequest{
			Name:                   credsName,
			ScalewayAccessKey:      accessKey,
			ScalewaySecretKey:      secretKey,
			ScalewayProjectId:      projectId,
			ScalewayOrganizationId: organizationId,
		}).Execute()
		if err != nil {
			utils.PrintlnError(err)
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
			os.Exit(1)
		}
		return creds

	case qovery.CLOUDPROVIDERENUM_GCP:
		gcpCredentials, err := func() *promptui.Prompt {
			return &promptui.Prompt{
				Label:   "Enter your GCP JSON credentials (*base64* encoded)",
				Default: "",
			}
		}().Run()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		creds, resp, err := client.CloudProviderCredentialsAPI.CreateGcpCredentials(context.Background(), orgaId).GcpCredentialsRequest(qovery.GcpCredentialsRequest{
			Name:           credsName,
			GcpCredentials: gcpCredentials,
		}).Execute()
		if err != nil {
			utils.PrintlnError(err)
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
			os.Exit(1)
		}
		return creds
	case qovery.CLOUDPROVIDERENUM_ON_PREMISE:
		creds, resp, err := client.CloudProviderCredentialsAPI.CreateOnPremiseCredentials(context.Background(), orgaId).OnPremiseCredentialsRequest(qovery.OnPremiseCredentialsRequest{
			Name: "on-premise",
		}).Execute()
		if err != nil {
			utils.PrintlnError(err)
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
			os.Exit(1)
		}
		return creds
	}

	panic("Unhandled cloud provider type during credentials creation")
}

func configureStorageClass(client *qovery.APIClient, cluster *qovery.Cluster) {
	if cluster.CloudProvider != qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		return
	}

	utils.Println("We need to know the storage class name that your kubernetes cluster uses to deploy app with network storage.")
	storageClassUI := promptui.Select{
		Label: "Storage class name",
	}
	_, storageClassName, err := storageClassUI.Run()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	settings, _, err := client.ClustersAPI.GetClusterAdvancedSettings(context.Background(), cluster.Organization.Id, cluster.Id).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	settings.StorageclassFastSsd = &storageClassName
	_, _, err = client.ClustersAPI.EditClusterAdvancedSettings(context.Background(), cluster.Organization.Id, cluster.Id).ClusterAdvancedSettings(*settings).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
}

func configureRegistry(client *qovery.APIClient, cluster *qovery.Cluster) {
	if cluster.CloudProvider != qovery.CLOUDPROVIDERENUM_ON_PREMISE {
		return
	}

	configureContainerRegistryPrompt := promptui.Select{
		Label: "You need to configure a container registry that Qovery will use to push images for your cluster. Do you want to do it now ?",
		Items: []string{"Yes", "No"},
	}

	_, configureContainerRegistry, err := configureContainerRegistryPrompt.Run()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	if configureContainerRegistry == "No" {
		return
	}

	resp, _, err := client.ContainerRegistriesAPI.ListContainerRegistry(context.Background(), cluster.Organization.Id).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	ix := slices.IndexFunc(resp.GetResults(), func(c qovery.ContainerRegistryResponse) bool { return c.Cluster != nil && c.Cluster.Id == cluster.Id })
	cr := resp.Results[ix]

	url, err := func() *promptui.Prompt {
		return &promptui.Prompt{
			Label:   "Url of your registry",
			Default: "https://",
		}
	}().Run()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	login, err := func() *promptui.Prompt {
		return &promptui.Prompt{
			Label:   "Username to use to login to your registry",
			Default: "",
		}
	}().Run()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	password, err := func() *promptui.Prompt {
		return &promptui.Prompt{
			Label:   "Password to use to login to your registry",
			Default: "",
		}
	}().Run()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	_, res, err := client.ContainerRegistriesAPI.EditContainerRegistry(context.Background(), cluster.Organization.Id, cr.Id).ContainerRegistryRequest(qovery.ContainerRegistryRequest{
		Name:        *cr.Name,
		Kind:        *cr.Kind,
		Description: cr.Description,
		Url:         &url,
		Config: qovery.ContainerRegistryRequestConfig{
			Username: &login,
			Password: &password,
		},
	}).Execute()

	if err != nil {
		utils.PrintlnError(err)
		body, _ := io.ReadAll(res.Body)
		fmt.Printf("%s: %v\n", color.RedString("Error"), string(body))
		os.Exit(1)
	}
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

func promptForClusterName(defaultName string) string {
	utils.Println("Cluster Name:")
	clusterNamePrompt := promptui.Prompt{
		Label:   "Give a name to your new cluster",
		Default: defaultName,
	}
	mClusterName, err := clusterNamePrompt.Run()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
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
	}
	return string(helmValuesYamlBytes)
}

func getBaseHelmValuesContent(kubernetesType qovery.CloudProviderEnum) string {
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
		utils.PrintlnError(err)
		os.Exit(1)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	// Check server response
	if res.StatusCode != http.StatusOK {
		utils.PrintlnError(fmt.Errorf("bad status while downloading Qovery Helm Values file: %s", res.Status))
		os.Exit(1)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	return string(body)
}

func init() {
	clusterCmd.AddCommand(clusterInstallCmd)
}
