package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var (
	userInfo string
	organizationName string
	userErr error
	orgaErr error
	adminUserAddCmd = &cobra.Command{
		Use: "add",
		Short: "Add a user in an organization",
		Long: "Add user with his mail or subId to an organization with it's name",
		Run: func(cmd *cobra.Command, args []string) { addUserToOrganization() },
	}
	)

func init() {
	adminUserAddCmd.Flags().StringVarP(&userInfo, "user", "u", "", "User's mail or SubId")
	userErr = adminUserAddCmd.MarkFlagRequired("user")
	adminUserAddCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization's name")
	orgaErr = adminUserAddCmd.MarkFlagRequired("organization")
	adminCmd.AddCommand(adminUserAddCmd)
}

func addUserToOrganization(){
	if userErr != nil {
		log.Error("Invalid user info")
	} else if orgaErr != nil {
		log.Error("Invalid organization name")
	} else {
		io.AddUserToOrganization(organizationName, userInfo)
	}
}
