package cmd

import (
	_ "embed"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var (
	demoClusterName        string
	demoDeleteQoveryConfig bool
)

//go:embed demo_scripts/create_qovery_demo.sh
var demoScriptsCreate []byte

//go:embed demo_scripts/destroy_qovery_demo.sh
var demoScriptsDestroy []byte

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Try Qovery on your local machine",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}
