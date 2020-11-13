package cmd

import (
	"github.com/spf13/cobra"
)

var organizationCmd = &cobra.Command{
	Use:   "organization",
	Short: "Perform organization actions",
	Long: `ORGANIZATION performs actions on organizations. For example:

	qovery organization list`,
}

func init() {
	RootCmd.AddCommand(organizationCmd)
}
