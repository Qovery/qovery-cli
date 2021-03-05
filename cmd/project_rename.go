package cmd

import (
	"fmt"
	"github.com/Qovery/qovery-cli/io"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var projectRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Perform project renaming",
	Long: `RENAME renames current project to the given name

qovery project rename [new_name] - renames current project to new_name 
`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, false, false, true)

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName, OrganizationName)

		project := io.RenameProject(p, args[0])

		if project.Name == args[0] {
			fmt.Println(color.GreenString("ok"))
			fmt.Println()
			fmt.Println("Your project has been renamed. Please, " +
				"update .qovery.yml configuration to match your new project name")
		} else {
			fmt.Println(color.YellowString("error"))
		}
	},
}

func init() {
	projectRenameCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	projectRenameCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	projectCmd.AddCommand(projectRenameCmd)
}
