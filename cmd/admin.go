package cmd

import (
	"github.com/spf13/cobra"
)

var (
	jwtKid           string
	clusterId        string
	organizationId   string
	projectId        string
	lockReason       string
	orgaErr          error
	dryRun           bool
	noConfirm        bool
	version          string
	versionErr       error
	ageInDay         int
	execId           string
	directory        string
	rootDns          string
	additionalClaims string
	description      string
	adminCmd         = &cobra.Command{Use: "admin", Hidden: true}
)



func init() {
	rootCmd.AddCommand(adminCmd)
}
