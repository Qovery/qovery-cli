package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Setup the CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		utils.PrintlnInfo("Current context:")
		utils.PrintlnContext()
		println()
		utils.PrintlnInfo("Select new context")
		err := utils.SelectOrganization()
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
		utils.PrintlnContext()
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
