package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
)

var(
	adminDeployAllCmd = &cobra.Command{
		Use: "deploy_all",
		Short: "Deploy all customers clusters",
		Run: func(cmd *cobra.Command, args []string){
			deployAllClusters()
		},
	}
)

func init() {
	adminDeployAllCmd.Flags().BoolVarP(&dryRun,"disable-dry-run", "y", false, "Disable dry run mode")
	adminCmd.AddCommand(adminDeployAllCmd)
}

func deployAllClusters() {
	pkg.DeployAll(dryRun)
}
