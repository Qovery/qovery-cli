package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"

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

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		clusterInstallBaseValuesFile, err = validateClusterInstallBaseValuesFile(clusterInstallBaseValuesFile)
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
		var selfManagedService = selfmanaged.NewSelfManagedClusterService(client, clusterService, clusterCredentialsService, containerRegistryService, promptUiFactory, clusterInstallBaseValuesFile)
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

var clusterInstallBaseValuesFile string

func validateClusterInstallBaseValuesFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return "", fmt.Errorf("expand base values file path: %w", err)
	}

	absPath, err := filepath.Abs(expandedPath)
	if err != nil {
		return "", fmt.Errorf("resolve base values file path: %w", err)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("read base values file path %q: %w", absPath, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("base values file path %q must be a file", absPath)
	}

	return absPath, nil
}

func init() {
	clusterInstallCmd.Flags().StringVar(&clusterInstallBaseValuesFile, "base-values-file", "", "Local Helm values base file to use instead of downloading values-demo-<provider>.yaml from qovery-chart")
	clusterCmd.AddCommand(clusterInstallCmd)
}
