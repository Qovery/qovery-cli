package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print installed version of the Qovery CLI",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		currentVersion, err := pkg.GetCurrentVersion()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.PrintlnInfo(fmt.Sprintf("%s\n", currentVersion))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
