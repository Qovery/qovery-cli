package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var demo2Cmd = &cobra.Command{
	Use:   "demo2",
	Short: "Try the experimental local demo based on Qovery Operator and Engine V2",
	Long: `Try the experimental local demo based on Qovery Operator and Engine V2.

This proof validates only:
  CLI -> Operator bootstrap -> heartbeat -> cluster deployment
      -> q-core compiles the current catalog -> RUN_ONCE worker installs the platform

It does not yet validate complete legacy demo catalog parity, ingress-nginx, application builds or
deployments, or the complete heartbeat compatibility contract that must later be enforced by
q-core.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		if len(args) == 0 {
			_ = cmd.Help()
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(demo2Cmd)
}
