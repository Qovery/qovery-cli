package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var getTokenCommand = &cobra.Command{
	Use:   "get-token",
	Short: "Get token for a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		getToken()
	},
}

func init() {
	getTokenCommand.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	clusterCmd.AddCommand(getTokenCommand)
}

func getToken() {
	response := pkg.GetTokenByClusterId(clusterId)
	utils.Println(response)
}
