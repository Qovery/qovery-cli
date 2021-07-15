package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage Qovery CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.PrintlnInfo("Current context:")
		err := utils.PrintlnContext()
		if err != nil {
			fmt.Println("Context not yet configured. ")
		}
		println()
		utils.PrintlnInfo("You can set a new context using 'qovery context set'. ")
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
