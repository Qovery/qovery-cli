package cmd

import (
	"encoding/json"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/xeonx/timeago"
	"io/ioutil"
	"net/http"
	"os"
	"qovery-cli/io"
	"strings"
	"time"
)

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	Long: `LIST show all deployable environment. For example:

	qovery deploy list`,
	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, true, true, false)
		ShowDeploymentList(OrganizationName, ProjectName, BranchName, ApplicationName)
	},
}

func init() {
	deployListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	deployListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	deployListCmd.PersistentFlags().StringVarP(&BranchName, "branch", "b", "", "Your branch name")
	deployListCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "a", "", "Your application name")

	deployCmd.AddCommand(deployListCmd)
}

func ShowDeploymentList(organizationName string, projectName string, branchName string, applicationName string) {
	table := io.GetTable()
	table.SetHeader([]string{"branch", "commit date", "commit id", "commit message", "commit author", "deployed"})

	project := io.GetProjectByName(projectName, organizationName)
	environment := io.GetEnvironmentByName(project.Id, branchName, false)
	application := io.GetApplicationByName(project.Id, environment.Id, applicationName, false)

	if environment.Id == "" {
		table.Append([]string{"", "", "", "", "", ""})
		table.Render()
		return
	}

	var commits = getApplicationCommits(project.Id, environment.Id, application.Id)

	for _, commit := range commits {
		checkChar := ""
		if commit.Deployed {
			checkChar = color.GreenString("✓")
		}

		table.Append([]string{branchName, timeago.English.Format(commit.Timestamp), commit.Sha,
			strings.TrimSpace(commit.Message), commit.AuthorName, checkChar})

	}
	table.Render()
}

type ApplicationCommit struct {
	Sha        string    `json:"sha"`
	Deployed   bool      `json:"deployed"`
	Timestamp  time.Time `json:"timestamp"`
	Message    string    `json:"message"`
	AuthorName string    `json:"author_name"`
}

func getApplicationCommits(project string, environment string, application string) []ApplicationCommit {
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

	var r []ApplicationCommit
	body, _ := ioutil.ReadAll(res.Body)
	_ = json.Unmarshal(body, &r)

	return r
}
