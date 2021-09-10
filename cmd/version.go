package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print installed version of the Qovery CLI",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.PrintlnInfo(fmt.Sprintf("%s\n", pkg.GetCurrentVersion()))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
