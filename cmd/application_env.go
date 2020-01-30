package cmd

import (
	"github.com/spf13/cobra"
)

var applicationEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Perform application's environment variable actions",
	Long: `ENV performs application's environment variables actions. For example:

	qovery application env`,
}

func init() {
	applicationCmd.AddCommand(applicationEnvCmd)
}
