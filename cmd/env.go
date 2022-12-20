package cmd

import (
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage Environment Variables and Secrets",
}

func init() {
	rootCmd.AddCommand(envCmd)
}
