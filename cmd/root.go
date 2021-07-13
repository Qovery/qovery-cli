package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "qovery",
	Short: "A Command-line interface of Qovery platform",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if !utils.QoveryContextExists() {
		err := utils.InitializeQoveryContext()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
