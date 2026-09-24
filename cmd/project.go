package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage Project",
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
