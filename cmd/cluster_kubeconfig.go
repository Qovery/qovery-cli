package cmd

import (
	"os"
	"path/filepath"

	"github.com/qovery/qovery-cli/pkg"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var downloadKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Retrieve kubeconfig with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		// Check if required flags are provided
		if clusterId == "" {
			_ = cmd.Help()
			os.Exit(0)
		}

		downloadKubeconfig()
	},
}

func init() {
	downloadKubeconfigCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	clusterCmd.AddCommand(downloadKubeconfigCmd)
}

func downloadKubeconfig() {
	// download kubeconfig
	kubeconfig := pkg.GetKubeconfigByClusterId(clusterId)

	// get current working directory
	dir, err := os.Getwd()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	kubeconfigFilename := filepath.Join(dir, "kubeconfig-"+clusterId+".yaml")
	// create a file in the current folder
	writeError := os.WriteFile(kubeconfigFilename, []byte(kubeconfig), 0600)
	if writeError != nil {
		utils.PrintlnError(writeError)
		os.Exit(1)
	}

	log.Info("Kubeconfig file created in the current directory.")
	log.Info("Execute `export KUBECONFIG=" + kubeconfigFilename + "` to use it.")
}
