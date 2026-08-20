package cmd

import (
	"github.com/spf13/cobra"
)

var organizationCmd = &cobra.Command{
	Use:   "organization",
	Short: "Manage Organization",
}

func init() {
	rootCmd.AddCommand(organizationCmd)
}
