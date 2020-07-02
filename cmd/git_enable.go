package cmd

import (
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4"
	"net/http"
	"os"
	"qovery.go/io"
	"strings"
)

var gitEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enables git - Qovery webhooks in given git project",
	Long: `Enables git - Qovery webhooks in given project e.g.
qovery git enable
enables sending notifications about events in remote git repository (determined by your current working directory)`,
	Run: func(cmd *cobra.Command, args []string) {
		repo, _ := git.PlainOpen(".git")
		cfg, _ := repo.Config()
		url := cfg.Raw.Section("remote").Subsection("origin").Option("url")
		group, projectName := sanitize(url)
		enableWebhooks(group, projectName)
	},
}

func sanitize(repoUrl string) (group string, projectName string) {
	if strings.Contains(repoUrl, "@gitlab.com/") {
		prefixAndSuffix := strings.Split(repoUrl, "@gitlab.com/")

		if len(prefixAndSuffix) != 2 {
			printErrorAndQuit()
		}

		suffix := prefixAndSuffix[1]
		split := strings.Split(suffix, "/")

		if len(prefixAndSuffix) != 2 {
			printErrorAndQuit()
		}

		return split[0], strings.ReplaceAll(split[1], ".git", "")
	} else if strings.Contains(repoUrl, "git@gitlab.com:") {
		suffix := strings.ReplaceAll(repoUrl, "git@gitlab.com:", "")

		split := strings.Split(suffix, "/")

		if len(split) != 2 {
			printErrorAndQuit()
		}

		return split[0], strings.ReplaceAll(split[1], ".git", "")
	} else if strings.Contains(repoUrl, "https://gitlab.com/") {
		suffix := strings.ReplaceAll(repoUrl, "https://gitlab.com/", "")

		split := strings.Split(suffix, "/")

		if len(split) != 2 {
			printErrorAndQuit()
		}

		return split[0], strings.ReplaceAll(split[1], ".git", "")
	} else {
		println("This command is currently supported for Gitlab projects only. ")
		os.Exit(1)
		return "", ""
	}
}

func printErrorAndQuit() {
	println("Could not determine remote git repository URL.\n")
	println("Try running:")
	println("git config --get remote.origin.url\n")
	println("to make sure your local git repository is connected to a remote or contact #support on our Discord - https://discord.qovery.com")
	os.Exit(1)
}

func init() {
	gitCmd.AddCommand(gitEnableCmd)
}

func enableWebhooks(group string, projectName string) {
	token := io.GetAuthorizationToken()
	client := &http.Client{}
	url := io.RootURL + "/hook/gitlab/enable?group=" + group + "&projectName=" + projectName
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)

	if err != nil || res.StatusCode != 204 {
		println("Could not enable Qovery in " + group + "/" + projectName)
		os.Exit(1)
	}

	println("Enabled Qovery in " + group + "/" + projectName)
}
