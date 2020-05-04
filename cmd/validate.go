package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var validateCmd = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"valid"},
	Short:   "Validate the current config is valid",
	Long:    `Validate the Dockerfile and Qovery configuration file`,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := util.CurrentQoveryYML()
		if err != nil {
			util.PrintError("No qovery configuration file found")
			os.Exit(1)
		}

		for _, url := range util.ListRemoteURLs() {
			gas := api.GitCheck(url)

			if gas.HasAccess {
				println(color.GreenString("Access to " + gas.GitURL + " : OK"))

			} else {
				println(color.RedString("Access to " + gas.GitURL + " : KO"))
			}
		}

		println("Your configuration is valid")
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
}
