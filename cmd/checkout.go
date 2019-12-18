package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout",
	Short: "Equivalent to 'git checkout' but with Qovery magic sauce",
	Long: `CHECKOUT performs 'git checkout' action and set Qovery properties to target the right environment . For example:

	qovery checkout`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("qovery checkout <branch>")
			os.Exit(1)
		}

		branch := args[0]
		// checkout branch
		util.Checkout(branch)

		LoadAndSaveLocalConfiguration()
	},
}

func init() {
	RootCmd.AddCommand(checkoutCmd)
}

func LoadAndSaveLocalConfiguration() {
	api.DeleteLocalConfiguration()

	qConf := util.CurrentQoveryYML()
	branchName := util.CurrentBranchName()
	projectName := qConf.Application.Project
	appName := qConf.Application.Name

	if branchName == "" || projectName == "" {
		fmt.Println("The current directory is not a Qovery project. Please consider using 'qovery init'")
		os.Exit(1)
	}

	project := api.GetProjectByName(projectName)
	if project == nil {
		fmt.Println("The project does not exist. Are you well authenticated with the right user? Do 'qovery auth' to be sure")
		os.Exit(1)
	}

	applications := api.ListApplicationsRaw(project.Id, branchName)
	if val, ok := applications["results"]; ok {
		results := val.([]interface{})
		for _, application := range results {
			a := application.(map[string]interface{})
			if a["name"] == appName {
				api.SaveLocalConfiguration(a)
				break
			}
		}
	}
}
