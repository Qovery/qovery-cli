package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Connect to an application container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.PrintlnInfo("Select organization")
		orga, err := utils.SelectOrganization()
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		utils.PrintlnInfo("Select project")
		project, err := utils.SelectProject(orga.ID)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		utils.PrintlnInfo("Select environment")
		env, err := utils.SelectEnvironment(project.ID)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		utils.PrintlnInfo("Select application")
		app, err := utils.SelectApplication(env.ID)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		pkg.ExecShell(&pkg.ShellRequest{
			ApplicationID:  app.ID,
			ProjectID:      project.ID,
			OrganizationID: orga.ID,
			EnvironmentID:  env.ID,
			ClusterID:      env.ClusterID,
		})
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
