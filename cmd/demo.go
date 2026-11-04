package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Try Qovery on your local machine",
	Long: `Try Qovery on your local machine.

qovery demo up creates a local k3d cluster, installs the Qovery Operator on it and lets Qovery
deploy the self-managed platform catalog with Engine v2.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
		if len(args) > 0 {
			utils.PrintlnError(fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath()))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}
