package cmd

import (
	"github.com/spf13/cobra"
)

var gitlabCmd = &cobra.Command{
	Use:   "gitlab",
	Short: "Manage your Gitlab projects",
	Long:  `GITLAB enables you to configure your Gitlab projects with Qovery`,
}

func init() {
	RootCmd.AddCommand(gitlabCmd)
}
