package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "qovery",
	Short: "A Command-line Interface of the Qovery platform",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(0)
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
			os.Exit(0)
		}
	}
}
