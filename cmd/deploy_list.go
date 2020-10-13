package cmd

import (
	"encoding/json"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/xeonx/timeago"
	"io/ioutil"
	"net/http"
	"os"
	"qovery.go/io"
	"strings"
)

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	Long: `LIST show all deployable environment. For example:

	qovery deploy list`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = io.CurrentBranchName()
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
			ApplicationName = qoveryYML.Application.GetSanitizeName()
		}

		ShowDeploymentList(ProjectName, BranchName, ApplicationName)
	},
}

func init() {
	deployListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	deployListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	deployListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	deployCmd.AddCommand(deployListCmd)
}

func ShowDeploymentList(projectName string, branchName string, applicationName string) {
	table := io.GetTable()
	table.SetHeader([]string{"branch", "commit date", "commit id", "commit message", "commit author", "deployed"})

	project := io.GetProjectByName(projectName)
	environment := io.GetEnvironmentByName(project.Id, branchName)
	application := io.GetApplicationByName(project.Id, environment.Id, applicationName)

	if environment.Id == "" {
		table.Append([]string{"", "", "", "", "", ""})
		table.Render()
		return
	}

	var deployedApplication = getDeployedCommit(project.Id, environment.Id, application.Id)

	// TODO param for n last commits
	for _, commit := range io.ListCommits(10) {
		checkChar := ""
		if deployedApplication.Commit == commit.ID().String() {
			checkChar = color.GreenString("✓")
		}

		table.Append([]string{branchName, timeago.English.Format(commit.Author.When), commit.ID().String(),
			strings.TrimSpace(commit.Message), commit.Author.Name, checkChar})
	}
	table.Render()
}

type LastDeployedApplication struct {
	Commit string `json:"commit"`
}

func getDeployedCommit(project string, environment string, application string) LastDeployedApplication {
	url := io.DefaultRootUrl + "/project/" + project + "/environment/" + environment + "/application/" + application + "/deployed"
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+io.GetAuthorizationToken())
	req.Header.Set("content-type", "application/json")
	res, err := client.Do(req)

	if err != nil {
		println("Error getting deployment list from Qovery")
		os.Exit(1)
	}

	r := LastDeployedApplication{}
	body, _ := ioutil.ReadAll(res.Body)
	_ = json.Unmarshal(body, &r)

	return r
}
