package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminUpdateAllCmd = &cobra.Command{
		Use:   "update_all",
		Short: "Update all customers clusters to a specific version",
		Run: func(cmd *cobra.Command, args []string) {
			updateAllClusters()
		},
	}
)

func init() {
	adminUpdateAllCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	adminUpdateAllCmd.Flags().StringVarP(&version, "version", "v", "", "Targeted version")
	versionErr = adminUpdateAllCmd.MarkFlagRequired("version")
	adminCmd.AddCommand(adminUpdateAllCmd)
}

func updateAllClusters() {
	if versionErr != nil {
		log.Error("Invalid version")
		return
	}
	pkg.UpdateAll(dryRun, version)
}
