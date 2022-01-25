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
		_ = utils.ResetApplicationContext()
		utils.PrintlnInfo("Select new context")
		orga, err := utils.SelectAndSetOrganization()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		project, err := utils.SelectAndSetProject(orga.ID)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		env, err := utils.SelectAndSetEnvironment(project.ID)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		_, err = utils.SelectAndSetApplication(env.ID)
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
