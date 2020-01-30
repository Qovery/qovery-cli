package cmd

import (
	"github.com/spf13/cobra"
)

var environmentEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Perform environment's environment variable actions",
	Long: `ENV performs environment's environment variables actions. For example:

	qovery environment env`,
}

func init() {
	environmentCmd.AddCommand(environmentEnvCmd)
}
