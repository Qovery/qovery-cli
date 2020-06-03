package cmd

import (
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"qovery.go/io"
	"strings"
)

var gitlabEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enables Gitlab - Qovery webhooks in given project",
	Long: `Enables Gitlab - Qovery webhooks in given project e.g.
qovery gitlab enable https://gitlab.com/pjeziorowski/publicproject
makes Gitlab send notifications about events in https://gitlab.com/pjeziorowski/publicproject project`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			println("Usage: qovery gitlab enable <REPO_URL>")
			os.Exit(1)
		}
		group, projectName := sanitize(args[0])
		enableWebhooks(group, projectName)
	},
}

func sanitize(repoUrl string) (group string, projectName string) {
	repoWithoutPrefix := strings.ReplaceAll(repoUrl, "https://gitlab.com/", "")
	repoWithoutPrefixAndSuffix := strings.ReplaceAll(repoWithoutPrefix, ".git", "")
	split := strings.Split(repoWithoutPrefixAndSuffix, "/")

	if len(split) != 2 {
		println("Usage: qovery gitlab enable <REPO_URL>")
		println("where <REPO_URL> is URL to your Gitlab project e.g.")
		println("https://gitlab.com/pjeziorowski/publicproject")
		os.Exit(1)
	}

	return split[0], split[1]
}

func init() {
	gitlabCmd.AddCommand(gitlabEnableCmd)
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
