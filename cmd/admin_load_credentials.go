package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	adminLoadCredentialsCmd = &cobra.Command{
		Use:   "load-credentials",
		Short: "Load credentials for a given cluster ID",
		Long: `This command is used to load credentials 
> Examples
----------
* Load credentials from a clusterID 12345678-1234-1234-1234-123456789012
qovery admin load-credentials --cluster-id 12345678-1234-1234-1234-123456789012

`,
		Run: func(cmd *cobra.Command, args []string) {
			err := pkg.LoadCredentials(clusterId, doNotConnectToBastion)
			utils.CheckError(err)
		},
	}
)

func init() {
	adminLoadCredentialsCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "ID of the cluster to load credentials for")
	adminLoadCredentialsCmd.Flags().BoolVarP(&doNotConnectToBastion, "no-bastion", "n", false, "do not connect to the bastion")
	adminCmd.AddCommand(adminLoadCredentialsCmd)
}
