package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/utils"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Qovery CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.PrintlnInfo("Current context:")
		err := utils.PrintlnContext()
		if err != nil {
			fmt.Println("Context not yet configured. ")
		}
		println()
		utils.PrintlnInfo("Select new context")
		err = utils.SelectOrganization()
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		id, _, _ := utils.CurrentOrganization()

		err = utils.SelectProject(id)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		id, _, _ = utils.CurrentProject()

		err = utils.SelectEnvironment(id)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		id, _, _ = utils.CurrentEnvironment()

		err = utils.SelectApplication(id)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		_, _, _ = utils.CurrentApplication()
		println()
		utils.PrintlnInfo("New context:")
		err = utils.PrintlnContext()
		if err != nil {
			utils.PrintlnError(err)
		}
	},
}

func init() {
	contextCmd.AddCommand(setCmd)
}
