package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	providerKind      string
	parallelRun       int
	providerErr       error
	adminUpdateAllCmd = &cobra.Command{
		Use:   "update_bulk",
		Short: "Update an amount of clusters to a specific version based on cloud provider kind.",
		Run: func(cmd *cobra.Command, args []string) {
			updateAllClusters()
		},
	}
)

func init() {
	adminUpdateAllCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	adminUpdateAllCmd.Flags().StringVarP(&version, "version", "v", "", "Targeted version")
	adminUpdateAllCmd.Flags().StringVarP(&providerKind, "provider-kind", "k", "", "Provider to upgrade. Can be : AWS, DO or SCW")
	adminUpdateAllCmd.Flags().IntVarP(&parallelRun, "parallel-run", "p", 1, "Number of parallel upgrades. Max is 20.")
	versionErr = adminUpdateAllCmd.MarkFlagRequired("version")
	providerErr = adminUpdateAllCmd.MarkFlagRequired("provider-kind")
	adminCmd.AddCommand(adminUpdateAllCmd)
}

func updateAllClusters() {
	if versionErr != nil {
		log.Error("Invalid version")
		return
	}
	if providerErr != nil {
		log.Error("Provider kind is mandatory")
		return
	}

	if parallelRun > 20 {
		log.Error("Can't update more than 20 clusters")
	}
	pkg.UpdateAll(dryRun, version, providerKind, parallelRun)
}
