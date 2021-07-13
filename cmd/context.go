package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Setup the CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		utils.PrintlnInfo("Select new context")
		err := utils.SelectOrganization()
		if err != nil {
			panic(err)
		}
		id, _, _ := utils.CurrentOrganization()

		err = utils.SelectProject(id)
		if err != nil {
			panic(err)
		}
		id, _, _ = utils.CurrentProject()

		err = utils.SelectEnvironment(id)
		if err != nil {
			panic(err)
		}
		id, _, _ = utils.CurrentEnvironment()

		err = utils.SelectApplication(id)
		if err != nil {
			panic(err)
		}
		_, _, _ = utils.CurrentApplication()
		utils.PrintlnContext()
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
