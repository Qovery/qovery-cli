package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
)

var projectRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Perform project renaming",
	Long: `RENAME renames current project to the given name

qovery project rename [new_name] - renames current project to new_name 
`,
	Run: func(cmd *cobra.Command, args []string) {
		qoveryYML, err := io.CurrentQoveryYML()
		if err != nil {
			io.PrintError("No qovery configuration file found")
			os.Exit(1)
		}
		ProjectName = qoveryYML.Application.Project

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		p := io.GetProjectByName(ProjectName)

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
	projectCmd.AddCommand(projectRenameCmd)
}
