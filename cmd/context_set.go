package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Qovery CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		err := utils.SetContext(true, true, true, true)
		if err != nil {
			utils.PrintlnError(err)
		}
	},
}

func init() {
	contextCmd.AddCommand(setCmd)
}
