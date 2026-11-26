package cmd

import (
	"strings"
	"testing"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

func storedApplicationRequest(autoDeploy *bool, autoPreview *bool) qovery.ApplicationEditRequest {
	name := "app"
	return qovery.ApplicationEditRequest{
		Name:        &name,
		AutoPreview: autoPreview,
		AutoDeploy:  *qovery.NewNullableBool(autoDeploy),
	}
}

func TestApplicationUpdateSendsAutoPreviewWhenGiven(t *testing.T) {
	setFlagForTest(t, applicationUpdateCmd, "auto-preview", "true")
	req := storedApplicationRequest(utils.Bool(true), utils.Bool(false))

	applyApplicationAutoFlags(applicationUpdateCmd, &req)

	body := requestBody(t, req)
	if body["auto_preview"] != true || body["auto_deploy"] != true {
		t.Fatalf("expected auto_preview true and the stored auto_deploy true, got %v", body)
	}
}

func TestApplicationUpdateTurnsAutoPreviewOff(t *testing.T) {
	setFlagForTest(t, applicationUpdateCmd, "auto-preview", "false")
	req := storedApplicationRequest(nil, utils.Bool(true))

	applyApplicationAutoFlags(applicationUpdateCmd, &req)

	body := requestBody(t, req)
	if body["auto_preview"] != false {
		t.Fatalf("expected auto_preview false, got %v", body)
	}
}

func TestApplicationUpdateKeepsStoredAutoPreviewWhenNotGiven(t *testing.T) {
	for _, stored := range []*bool{utils.Bool(true), utils.Bool(false), nil} {
		req := storedApplicationRequest(nil, stored)
		expected := requestBody(t, req)

		applyApplicationAutoFlags(applicationUpdateCmd, &req)

		body := requestBody(t, req)
		if body["auto_preview"] != expected["auto_preview"] {
			t.Fatalf("expected stored auto_preview %v, got %v", expected["auto_preview"], body)
		}
	}
}

func TestApplicationUpdateStillSendsAutoDeployWhenGiven(t *testing.T) {
	setFlagForTest(t, applicationUpdateCmd, "auto-deploy", "false")
	req := storedApplicationRequest(utils.Bool(true), utils.Bool(true))

	applyApplicationAutoFlags(applicationUpdateCmd, &req)

	body := requestBody(t, req)
	if body["auto_deploy"] != false || body["auto_preview"] != true {
		t.Fatalf("expected auto_deploy false and the stored auto_preview true, got %v", body)
	}
}

func gitApplication() *qovery.Application {
	return &qovery.Application{
		Id:   "app-git",
		Name: "api",
		GitRepository: &qovery.ApplicationGitRepository{
			Provider:         qovery.GITPROVIDERENUM_GITHUB,
			Url:              "https://github.com/acme/api.git",
			Branch:           qovery.PtrString("main"),
			RootPath:         qovery.PtrString("/"),
			DeployedCommitId: qovery.PtrString("4f2c1e0"),
		},
	}
}

func applicationWithoutGitRepository() *qovery.Application {
	return &qovery.Application{
		Id:          "app-image",
		Name:        "web",
		Description: qovery.PtrString("serves the site"),
		AutoPreview: utils.Bool(false),
	}
}

func TestApplicationUpdateWithoutGitRepositoryKeepsTheOtherFields(t *testing.T) {
	req, err := applicationEditRequest(applicationWithoutGitRepository(), "")
	if err != nil {
		t.Fatal(err)
	}

	body := requestBody(t, req)
	if _, sent := body["git_repository"]; sent {
		t.Fatalf("expected no git_repository for an application without one, got %v", body)
	}
	if body["name"] != "web" || body["description"] != "serves the site" || body["auto_preview"] != false {
		t.Fatalf("expected the stored fields to be kept, got %v", body)
	}
}

func TestApplicationUpdateBranchWithoutGitRepositoryIsRejected(t *testing.T) {
	_, err := applicationEditRequest(applicationWithoutGitRepository(), "feature")
	if err == nil {
		t.Fatal("expected an error for --branch on an application without Git repository")
	}
	for _, expected := range []string{"web", "not built from a Git repository", "--branch"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected %q in the error, got %v", expected, err)
		}
	}
}

func TestApplicationUpdateBranchOnGitApplicationChangesOnlyTheBranch(t *testing.T) {
	req, err := applicationEditRequest(gitApplication(), "feature")
	if err != nil {
		t.Fatal(err)
	}

	repository := requestBody(t, req)["git_repository"].(map[string]interface{})
	if repository["branch"] != "feature" || repository["url"] != "https://github.com/acme/api.git" || repository["root_path"] != "/" {
		t.Fatalf("expected the stored repository on branch feature, got %v", repository)
	}
}
