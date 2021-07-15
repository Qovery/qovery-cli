package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/io"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print installed version of the Qovery CLI",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture("version")
		utils.PrintlnInfo(fmt.Sprintf("%s\n", io.GetCurrentVersion()))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
