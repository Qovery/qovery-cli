package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var terraformId string
var terraformSetupBackendCmd = &cobra.Command{
	Use:   "setup-backend",
	Short: "Generate a Terraform backend configuration file that can be used to access your tf-state",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()

		// Retrieve terraform service and its environment
		terraform, _, err := client.TerraformMainCallsAPI.GetTerraform(context.Background(), terraformId).Execute()
		checkError(err)
		env, _, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), terraform.Environment.Id).Execute()
		checkError(err)
		utils.Println(fmt.Sprintf("Preparing backend.tf file for terraform `%s` of environment `%s`", terraform.Name, env.Name))

		// Download kubeconfig to connect to the cluster
		kubeconfigPath := downloadKubeconfig(env.ClusterId)

		// Create kubeclient to retrieve the namespace of the tfstate secret
		kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		checkError(err)
		kubeClient, err := kubernetes.NewForConfig(kubeconfig)
		checkError(err)
		secrets, err := kubeClient.CoreV1().Secrets("").List(context.Background(), v1.ListOptions{
			LabelSelector: fmt.Sprintf("qovery.com/service-id=%s,tfstate=true", terraform.Id),
		})
		checkError(err)
		if len(secrets.Items) == 0 {
			log.Errorf("No tfstate secret found for terraform %s. The service must be deployed at least succesfully once", terraform.Id)
			os.Exit(1)
		}

		// Generate backend.tf file
		backendtf := fmt.Sprintf(`
terraform {
  backend "kubernetes" {
    secret_suffix  = "%s"
    namespace      = "%s"
    config_path    = "%s"
  }
}
`, terraform.Id, secrets.Items[0].Namespace, kubeconfigPath)

		utils.Println("Would you like to write `backend.tf` in current directory ?")
		if !utils.Validate("") {
			return
		}

		utils.Println("Writing `backend.tf` file in current directory")
		err = os.WriteFile("backend.tf", []byte(backendtf), 0600)
		checkError(err)
		var commandName string
		switch terraform.Engine {
		case qovery.TERRAFORMENGINEENUM_TERRAFORM:
			commandName = "terraform"
		case qovery.TERRAFORMENGINEENUM_OPEN_TOFU:
			commandName = "tofu"
		}

		utils.Println(fmt.Sprintf("You can now run `%s init` to initialize your project with your tf-state configured on your cluster", commandName))
	},
}

func init() {
	terraformCmd.AddCommand(terraformSetupBackendCmd)
	terraformSetupBackendCmd.Flags().StringVarP(&terraformId, "terraform", "t", "", "Terraform UUID. If not provided, the CLI will use the service context")
}
