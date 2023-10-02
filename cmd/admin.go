package cmd

import (
	"github.com/spf13/cobra"
)

var (
	clusterId  string
	projectId  string
	lockReason string
	orgaErr    error
	dryRun     bool
	version    string
	versionErr error
	ageInDay int
	adminCmd   = &cobra.Command{Use: "admin", Hidden: true}
)

func init() {
	rootCmd.AddCommand(adminCmd)
}
