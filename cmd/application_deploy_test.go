package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestApplicationDeployCommitWithoutGitRepositoryIsRejectedBeforeSending(t *testing.T) {
	for _, commitId := range []string{"9b8a7c6", "latest"} {
		client, requests := newRecordingQoveryClient(t)

		err := utils.DeployApplications(client, "env-1", []*qovery.Application{applicationWithoutGitRepository()}, commitId)
		if err == nil {
			t.Fatalf("expected an error for commit %s on an application without Git repository", commitId)
		}
		for _, expected := range []string{"web", "not built from a Git repository", commitId} {
			if !strings.Contains(err.Error(), expected) {
				t.Errorf("expected %q in the error, got %v", expected, err)
			}
		}
		if len(*requests) != 0 {
			t.Fatalf("expected no request, got %v", *requests)
		}
	}
}
