package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"strings"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
	Long: `LIST show all available databases within a project and environment. For example:

	qovery database list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowDatabaseList(ProjectName, BranchName, ShowCredentials)
	},
}

func init() {
	databaseListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	databaseListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	databaseListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")

	databaseCmd.AddCommand(databaseListCmd)
}

func ShowDatabaseList(projectName string, branchName string, showCredentials bool) {
	table := io.GetTable()
	table.SetHeader([]string{"database name", "status", "type", "version", "endpoint", "port", "username", "password", "applications"})

	projectId := io.GetProjectByName(projectName).Id
	environment := io.GetEnvironmentByName(projectId, branchName)

	services := io.ListDatabases(projectId, environment.Id)
	if services.Results == nil || len(services.Results) == 0 {
		table.Append([]string{"", "", "", "", "", "", "", "", ""})
	} else {
		for _, a := range services.Results {
			applicationName := "none"
			if a.Applications != nil {
				applicationName = strings.Join(a.GetApplicationNames(), ", ")
			}

			endpoint := "<hidden>"
			port := "<hidden>"
			username := "<hidden>"
			password := "<hidden>"

			if showCredentials {
				endpoint = a.FQDN
				port = intPointerValue(a.Port)
				username = a.Username
				password = a.Password
			}

			table.Append([]string{
				a.Name,
				a.Status.GetColoredCodeMessage(),
				a.Type,
				a.Version,
				endpoint,
				port,
				username,
				password,
				applicationName,
			})
		}
	}
	table.Render()
	fmt.Printf("\n")
}
