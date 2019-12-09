package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information for the Qovery CLI",
	Long: `VERSION allows you to print version information for the qovery-cli. For example:

	qovery version`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("version called")
		showVersion()
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

func showVersion() {
	fmt.Println("Qovery version 1.0.0b")
}
