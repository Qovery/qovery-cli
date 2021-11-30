package cmd

import (
	"github.com/spf13/cobra"
)

var (
	clusterId string
	orgaErr error
	dryRun bool
	version string
	versionErr error
	adminCmd = &cobra.Command{Use: "admin", Hidden: true}
)

func init() {
	rootCmd.AddCommand(adminCmd)
}
