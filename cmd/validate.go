package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var validateCmd = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"valid"},
	Short:   "Validate the current config is valid",
	Long:    `Validate the Dockerfile and Qovery configuration file`,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := io.CurrentQoveryYML(BranchName)
		if err != nil {
			io.PrintError("No qovery configuration file found")
			os.Exit(1)
		}

		showRemoteRepositoryAccess()

		println("\nYour configuration is valid")
	},
}

func showRemoteRepositoryAccess() {
	for _, url := range io.ListRemoteURLs() {
		println(fmt.Sprintf("Check repository access to %s", url))
		gas := io.GitCheck(url)

		if gas.HasAccess {
			println(color.GreenString("OK"))

		} else {
			io.PrintError("Qovery can't access your repository.")
			io.PrintHint("Give access to Qovery to deploy your application. https://docs.qovery.com/docs/using-qovery/interface/cli")
		}
	}
}

func init() {
	RootCmd.AddCommand(validateCmd)
}
