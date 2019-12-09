package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"qovery.go/util"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Do project initialization to use Qovery",
	Long: `INIT do project initialization to use Qovery within the current directory. For example:

	qovery init`,
	Run: func(cmd *cobra.Command, args []string) {

		// TODO check that .qovery.yml does not exists
		// TODO check if Dockerfile exists or not
		// TODO ask if the

		p := util.QoveryYML{}

		p.Application.Project = util.AskForInput(false, "Enter the project name")
		p.Application.Name = util.AskForInput(false, "Enter the application name")
		p.Application.PubliclyAccessible = util.AskForConfirmation(false, "Would you like to expose publicly your application?", "y")

		count := 1
		for count < 100 {
			addDatabase := false
			if count == 1 {
				addDatabase = askForAddDatabase(true)
			} else {
				addDatabase = askForAddDatabase(false)
			}

			if addDatabase {
				// TODO add db
			} else {
				break
			}

			count++
		}

		count = 1

		fmt.Println(p)
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func askForAddDatabase(firstTime bool) bool {
	if firstTime {
		return util.AskForConfirmation(false, "Do you need a database? (PostgreSQL, MySQL, MongoDB, ...)", "n")
	} else {
		return util.AskForConfirmation(false, "Do you need to add another database?", "n")
	}
}
