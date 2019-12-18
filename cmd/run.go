package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Equivalent to 'docker build' and 'docker run' but with Qovery magic sauce",
	Long: `RUN performs 'docker build' and 'docker run' action and set Qovery properties to target the right environment . For example:

	qovery run`,
	Run: func(cmd *cobra.Command, args []string) {
		branchName := util.CurrentBranchName()
		projectName := util.CurrentQoveryYML().Application.Project

		if branchName == "" || projectName == "" {
			fmt.Println("The current directory is not a Qovery project. Please consider using 'qovery init'")
			os.Exit(1)
		}

		// TODO check docker is running

		qConf := util.CurrentQoveryYML()
		appName := qConf.Application.Name

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
					j, _ := json.Marshal(a)
					runContainer("", string(j)) // TODO docker file content
					break
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
}

func runContainer(dockerfileContent string, applicationConfigurationJSON string) {
	// TODO
}
