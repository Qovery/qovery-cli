package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"qovery.go/util"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Aliases: []string{"valid"},
	Short: "Validate the current config is valid",
	Long: `Validate the Dockerfile and Qovery configuration file`,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := util.CurrentQoveryYML()
		if err != nil {
			util.PrintError("No qovery configuration file found")
			os.Exit(1)
		}
		println("Your configuration is valid")
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
}