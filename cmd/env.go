package cmd

import (
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:    "env",
	Short:  "Manage Qovery CLI Environment Variables and Secrets",
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(envCmd)
}
