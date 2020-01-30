package cmd

import (
	"github.com/spf13/cobra"
)

var projectEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Perform project's environment variable actions",
	Long: `ENV performs project's environment variables actions. For example:

	qovery project env`,
}

func init() {
	projectCmd.AddCommand(projectEnvCmd)
}
