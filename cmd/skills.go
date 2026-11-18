package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg/skills"
	"github.com/qovery/qovery-cli/utils"
)

var skillsProject bool

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage the Qovery skills for AI coding agents",
	Long: `Manage the Qovery skills, which teach AI coding agents (Claude Code, Cursor, OpenCode,
Gemini CLI and other tools supporting Agent Skills) to deploy, troubleshoot and optimize
applications on Qovery.

Source: https://github.com/Qovery/qovery-skills`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
	},
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install or update the Qovery skills",
	Long: `Install the Qovery skills, globally by default or in the current project with --project.
Run it again to update them to the latest version.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		if err := skills.Install(skillsProject, os.Stdout, os.Stderr); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

var skillsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show where the Qovery skills are installed",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		installations := skills.FindInstallations()
		if len(installations) == 0 {
			fmt.Println("The Qovery skills are not installed. Install them with 'qovery skills install'.")
			return
		}
		for _, installation := range installations {
			fmt.Printf("%s (version %s)\n", installation.Path, installation.Version)
		}
	},
}

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the Qovery skills from every location",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		if err := skills.Uninstall(os.Stdout, os.Stderr); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsStatusCmd)
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsInstallCmd.Flags().BoolVarP(&skillsProject, "project", "", false, "Install in the current project only")
}
