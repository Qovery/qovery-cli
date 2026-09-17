package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	auditLogCmd = &cobra.Command{
		Use:   "audit-log",
		Short: "Interact with audit logs",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(auditLogCmd)
}
