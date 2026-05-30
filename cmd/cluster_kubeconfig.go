package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/qovery/qovery-cli/pkg"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var readOnlyKubeconfig bool

var downloadKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Retrieve kubeconfig with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		validateKubeconfigFlags()
		kubeconfigFilename := downloadKubeconfig(clusterId, readOnlyKubeconfig)
		log.Info("Kubeconfig file created in the current directory.")
		log.Info("Execute `export KUBECONFIG=" + kubeconfigFilename + "` to use it.")
		if readOnlyKubeconfig {
			log.Info("This kubeconfig uses read-only access (ServiceAccount with view ClusterRole).")
		}
	},
}

func init() {
	downloadKubeconfigCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	downloadKubeconfigCmd.Flags().BoolVarP(&readOnlyKubeconfig, "read-only", "r", false, "Download a read-only kubeconfig backed by a Kubernetes service account with the view ClusterRole")
	clusterCmd.AddCommand(downloadKubeconfigCmd)
}

func validateKubeconfigFlags() {
	if clusterId == "" {
		utils.PrintlnError(fmt.Errorf("cluster ID is required (--cluster-id)"))
		os.Exit(1)
	}
}

func downloadKubeconfig(clusterId string, readOnly bool) string {
	kubeconfig := pkg.GetKubeconfigByClusterId(clusterId, readOnly)

	dir, err := os.Getwd()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	suffix := ""
	if readOnly {
		suffix = "-readonly"
	}
	kubeconfigFilename := filepath.Join(dir, "kubeconfig"+suffix+"-"+clusterId+".yaml")
	writeError := os.WriteFile(kubeconfigFilename, []byte(kubeconfig), 0600)
	if writeError != nil {
		utils.PrintlnError(writeError)
		os.Exit(1)
	}

	return kubeconfigFilename
}
