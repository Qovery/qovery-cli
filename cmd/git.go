package cmd

import (
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Manage your git projects",
	Long:  `git enables you to configure your git projects with Qovery`,
}

func init() {
	RootCmd.AddCommand(gitCmd)
}
