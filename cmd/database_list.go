package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/xeonx/timeago"
	"qovery-cli/io"
)

var databaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases",
	Long: `LIST show all available databases within a project and environment. For example:

	qovery database list`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, false, false)
		ShowDatabaseListWithProjectAndBranchNames(OrganizationName, ProjectName, BranchName, ShowCredentials)
	},
}

func init() {
	databaseListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	databaseListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	databaseListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	databaseListCmd.PersistentFlags().BoolVarP(&ShowCredentials, "credentials", "c", false, "Show credentials")

	databaseCmd.AddCommand(databaseListCmd)
}

func ShowDatabaseListWithProjectAndBranchNames(organizationName string, projectName string, branchName string, showCredentials bool) {
	projectId := io.GetProjectByName(projectName, organizationName).Id
	environment := io.GetEnvironmentByName(projectId, branchName, true)
	databases := io.ListDatabases(projectId, environment.Id)
	ShowDatabaseList(databases.Results, showCredentials)
}

func ShowDatabaseList(databases []io.Service, showCredentials bool) {
	table := io.GetTable()
	table.SetHeader([]string{"database name", "status", "last update", "type", "version", "endpoint", "port", "username", "password"})

	if len(databases) == 0 {
		table.Append([]string{"", "", "", "", "", "", "", "", ""})
	} else {
		for _, a := range databases {
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
				a.Status.GetColoredStatus(),
				timeago.English.Format(a.UpdatedAt),
				a.Type,
				a.Version,
				endpoint,
				port,
				username,
				password,
			})
		}
	}
	table.Render()
	fmt.Printf("\n")
}
