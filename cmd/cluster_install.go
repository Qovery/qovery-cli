package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/cluster/containerregistry"
	"github.com/qovery/qovery-cli/pkg/cluster/credentials"
	"github.com/qovery/qovery-cli/pkg/cluster/selfmanaged"
	"github.com/qovery/qovery-cli/pkg/filewriter"
	"github.com/qovery/qovery-cli/pkg/organization"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
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
		var promptUiFactory promptuifactory.PromptUiFactory = &promptuifactory.PromptUiFactoryImpl{}
		var organizationService = organization.NewOrganizationService(client, promptUiFactory)
		var clusterService = cluster.NewClusterService(client, promptUiFactory)
		var clusterCredentialsService = credentials.NewClusterCredentialsService(client, promptUiFactory)
		var containerRegistryService = containerregistry.NewClusterContainerRegistryService(client, promptUiFactory)
		var selfManagedService = selfmanaged.NewSelfManagedClusterService(client, clusterService, clusterCredentialsService, containerRegistryService, promptUiFactory)
		var fileWriterService filewriter.FileWriterService = filewriter.NewFileWriterService()
		var service = selfmanaged.NewInstallSelfManagedClusterService(organizationService, selfManagedService, clusterService, fileWriterService, promptUiFactory)

		// when
		informationMessage, err := service.InstallCluster()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		if informationMessage != nil {
			utils.Println(fmt.Sprintf("%s\n", *informationMessage))
			os.Exit(0)
		}
	},
}

func init() {
	clusterCmd.AddCommand(clusterInstallCmd)
}
