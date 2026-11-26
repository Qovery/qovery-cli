package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

func TestApplicationDeployWithoutGitRepositorySendsNoCommit(t *testing.T) {
	client, requests := newRecordingQoveryClientRespondingWith(t, http.StatusAccepted, `{"id":"env-1","state":"DEPLOYING","last_deployment_state":"DEPLOYED"}`)

	err := utils.DeployApplications(client, "env-1", []*qovery.Application{applicationWithoutGitRepository(), gitApplication()}, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(*requests) != 1 || (*requests)[0].path != "/environment/env-1/service/deploy" {
		t.Fatalf("expected one deploy request, got %v", *requests)
	}
	var body struct {
		Applications []map[string]interface{} `json:"applications"`
	}
	if err := json.Unmarshal([]byte((*requests)[0].body), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Applications) != 2 {
		t.Fatalf("expected both applications, got %s", (*requests)[0].body)
	}
	if _, sent := body.Applications[0]["git_commit_id"]; sent || body.Applications[0]["application_id"] != "app-image" {
		t.Fatalf("expected app-image without git_commit_id, got %v", body.Applications[0])
	}
	if body.Applications[1]["application_id"] != "app-git" || body.Applications[1]["git_commit_id"] != "4f2c1e0" {
		t.Fatalf("expected app-git at its deployed commit, got %v", body.Applications[1])
	}
}

func TestApplicationCommitWithoutGitRepositoryIsRejected(t *testing.T) {
	applications := []*qovery.Application{gitApplication(), applicationWithoutGitRepository()}
	for _, commitId := range []string{"9b8a7c6", "latest"} {
		for _, flag := range []string{"--commit-id", "--version"} {
			err := utils.CheckApplicationsCommitId(applications, commitId, flag)
			expected := "application web runs an image, not a Git repository: " + flag + " only applies to Git applications"
			if err == nil || err.Error() != expected {
				t.Errorf("expected %q for commit %s, got %v", expected, commitId, err)
			}
		}
	}
}

func TestApplicationCommitOnGitApplicationsOrNoCommitIsAccepted(t *testing.T) {
	if err := utils.CheckApplicationsCommitId([]*qovery.Application{gitApplication()}, "latest", "--commit-id"); err != nil {
		t.Errorf("expected a commit to be accepted for a Git application, got %v", err)
	}
	if err := utils.CheckApplicationsCommitId([]*qovery.Application{applicationWithoutGitRepository()}, "", "--commit-id"); err != nil {
		t.Errorf("expected no commit to be accepted for an application without Git repository, got %v", err)
	}
}
