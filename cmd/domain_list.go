package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List domains",
	Long: `LIST show all linked domains. For example:

	qovery domain list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ShowDomainList(ProjectName, BranchName)
	},
}

func init() {
	domainListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	domainListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	domainCmd.AddCommand(domainListCmd)
}

func ShowDomainList(projectName string, branchName string) {
	table := util.GetTable()
	table.SetHeader([]string{"branch", "domain", "status", "validation domain", "router name"})

	projectId := api.GetProjectByName(projectName).Id
	environment := api.GetEnvironmentByName(projectId, branchName)

	routers := api.ListRouters(projectId, environment.Id)
	if routers.Results == nil || len(routers.Results) == 0 {
		table.Append([]string{"", "", ""})
	} else {
		for _, r := range routers.Results {

			for _, cd := range r.CustomDomains {
				table.Append([]string{
					branchName,
					cd.Domain,
					cd.Status.GetColoredCodeMessage(),
					cd.GetValidationDomain(),
					r.Name,
				})
			}

			table.Append([]string{
				branchName,
				r.ConnectionURI,
				color.GreenString("live"),
				"none",
				r.Name,
			})
		}
	}

	table.Render()
	fmt.Printf("\n")
}
