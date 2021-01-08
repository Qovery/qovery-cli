package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"net/http"
	"os"
	"qovery-cli/io"
	"strings"
)

func init() {
	RootCmd.AddCommand(report)
}

var report = &cobra.Command{
	Use:   "report",
	Short: "Sends report to Qovery team",
	Long:  `REPORT sends debugging information to Qovery team to help solve issues`,
	Run: func(cmd *cobra.Command, args []string) {
		report := CreateReport()
		jsonPayload, _ := json.Marshal(&report)
		client := &http.Client{}
		req, _ := http.NewRequest("POST", io.DefaultRootUrl+"/zapier/report", strings.NewReader(string(jsonPayload)))
		req.Header.Set("Authorization", "Bearer "+io.GetAuthorizationToken())
		req.Header.Set("content-type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			fmt.Println("Error sending the report:")
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if res.StatusCode == 401 {
			fmt.Println("Authentication error. Could not send the report. Please, try to sign in again using 'qovery auth' command. ")
			os.Exit(1)
		}
		if res.StatusCode != 200 {
			fmt.Println("Received an unsuccessful response while sending the report. ")
			os.Exit(1)
		}
		fmt.Println("Done!")
	},
}

func CreateReport() Report {
	var url string
	var project io.Project
	var environment io.Environment
	var application io.Application

	if repo, err := git.PlainOpen(".git"); err == nil {
		repoConfig, err := repo.Config()
		if err != nil {
			fmt.Println("Could not read local git repository config.")
		} else {
			url = repoConfig.Raw.Section("remote").Subsection("origin").Option("url")
		}
	} else {
		fmt.Println("Could not add git repository details to the report.")
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Could not add working directory details to the report.")
	}

	accountId := io.GetAccount().Id
	cliVersion := io.GetCurrentVersion()
	gitStatus := io.GitCheck(strings.ReplaceAll(url, ".git", ""))
	token := io.GetAuthorizationToken()
	urls := io.ListRemoteURLs()
	branch := io.CurrentBranchName()
	commits := hash(io.ListCommits(10))
	yml, err := io.CurrentQoveryYML(branch)

	if err != nil {
		fmt.Println("Could not add Qovery config details to the report.")
	} else {
		project = io.GetProjectByName(yml.Application.Project, yml.Application.Organization)
		environment = io.GetEnvironmentByName(project.Id, branch)
		application = io.GetApplicationByName(project.Id, environment.Id, yml.Application.Name)
	}

	dockerfile := io.CurrentDockerfileContent()

	return Report{
		UserAccountId:          accountId,
		CliVersion:             cliVersion,
		GitAccess:              gitStatus,
		WorkingDir:             wd,
		Token:                  token,
		RepoUrl:                url,
		RemoteUrls:             urls,
		BranchName:             branch,
		LastCommits:            commits,
		Project:                project,
		Environment:            environment,
		Application:            application,
		LocalDockerfileContent: dockerfile,
		LocalQoveryConfig:      yml,
	}
}

func hash(commits []*object.Commit) []string {
	var h []string

	for _, commit := range commits {
		hash := &commit.Hash
		h = append(h, hash.String())
	}

	return h
}

type Report struct {
	UserAccountId          string
	CliVersion             string
	GitAccess              io.GitAccessStatus
	WorkingDir             string
	Token                  string
	RepoUrl                string
	RemoteUrls             []string
	BranchName             string
	LastCommits            []string
	Project                io.Project
	Environment            io.Environment
	Application            io.Application
	LocalDockerfileContent string
	LocalQoveryConfig      io.QoveryYML
}
