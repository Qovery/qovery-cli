package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qovery/qovery-client-go"
)

const gitContainerSourceJson = `{"git_repository":{"provider":"GITHUB","owner":"acme","url":"https://github.com/acme/api.git",` +
	`"name":"acme/api","branch":"main","dockerfile_path":"Dockerfile","deployed_commit_id":"4f2c1e0"}}`

const imageContainerSourceJson = `{"image":{"image_name":"nginx","tag":"1.27",` +
	`"registry":{"id":"registry-1","name":"Docker Hub","url":"https://docker.io","kind":"DOCKER_HUB"}}}`

type recordedRequest struct {
	method string
	path   string
	body   string
}

func newRecordingQoveryClient(t *testing.T) (*qovery.APIClient, *[]recordedRequest) {
	t.Helper()
	return newRecordingQoveryClientRespondingWith(t, http.StatusAccepted, "")
}

func newRecordingQoveryClientRespondingWith(t *testing.T, status int, responseBody string) (*qovery.APIClient, *[]recordedRequest) {
	t.Helper()
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, recordedRequest{method: request.Method, path: request.URL.Path, body: string(body)})
		if responseBody != "" {
			writer.Header().Set("Content-Type", "application/json")
		}
		writer.WriteHeader(status)
		_, _ = io.WriteString(writer, responseBody)
	}))
	t.Cleanup(server.Close)

	configuration := qovery.NewConfiguration()
	configuration.Servers = qovery.ServerConfigurations{{URL: server.URL}}
	return qovery.NewAPIClient(configuration), &requests
}

func containerWithSource(t *testing.T, id string, name string, sourceJson string) *qovery.ContainerResponse {
	t.Helper()
	updatedAt := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	container := &qovery.ContainerResponse{Id: id, Name: name, UpdatedAt: &updatedAt}
	if sourceJson == gitContainerSourceJson {
		container.ImageName = "z1234-api"
		container.Tag = "4f2c1e0"
		container.Registry = qovery.ContainerRegistryProviderDetailsResponse{Id: "registry-cluster", Name: "Cluster registry"}
	} else {
		container.ImageName = "nginx"
		container.Tag = "1.27"
		container.Registry = qovery.ContainerRegistryProviderDetailsResponse{Id: "registry-1", Name: "Docker Hub"}
	}
	if sourceJson != "" {
		var source qovery.ContainerSourceResponse
		if err := json.Unmarshal([]byte(sourceJson), &source); err != nil {
			t.Fatalf("cannot decode container source: %v", err)
		}
		container.Source = &source
	}
	return container
}

func TestContainerDeployCommitIdOnGitContainerSendsGitCommitId(t *testing.T) {
	client, requests := newRecordingQoveryClient(t)
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)

	err := deployContainers(client, "env-1", []*qovery.ContainerResponse{gitContainer}, "", "9b8a7c6")
	if err != nil {
		t.Fatal(err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected one request, got %v", *requests)
	}
	request := (*requests)[0]
	if request.method != http.MethodPost || request.path != "/container/container-git/deploy" {
		t.Fatalf("unexpected request %s %s", request.method, request.path)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(request.body), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["git_commit_id"] != "9b8a7c6" {
		t.Fatalf("expected only git_commit_id in the body, got %s", request.body)
	}
}

func TestContainerDeployTagOnGitContainerIsRejectedBeforeSending(t *testing.T) {
	client, requests := newRecordingQoveryClient(t)
	imageContainer := containerWithSource(t, "container-image", "web", imageContainerSourceJson)
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)

	err := deployContainers(client, "env-1", []*qovery.ContainerResponse{imageContainer, gitContainer}, "v2", "")
	if err == nil {
		t.Fatal("expected an error for a tag on a Git container")
	}
	for _, expected := range []string{"api", "Git container", "https://github.com/acme/api.git (branch main)", "--commit-id"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected %q in the error, got %v", expected, err)
		}
	}
	if len(*requests) != 0 {
		t.Fatalf("expected no request, got %v", *requests)
	}
}

func TestContainerDeployCommitIdOnImageContainerIsRejectedBeforeSending(t *testing.T) {
	client, requests := newRecordingQoveryClient(t)
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)
	imageContainer := containerWithSource(t, "container-image", "web", imageContainerSourceJson)

	err := deployContainers(client, "env-1", []*qovery.ContainerResponse{gitContainer, imageContainer}, "", "9b8a7c6")
	if err == nil {
		t.Fatal("expected an error for a commit on an image container")
	}
	for _, expected := range []string{"web", "nginx:1.27", "Docker Hub", "--commit-id only applies to Git containers"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("expected %q in the error, got %v", expected, err)
		}
	}
	if len(*requests) != 0 {
		t.Fatalf("expected no request, got %v", *requests)
	}
}

func TestContainerDeployRejectsTagAndCommitIdTogether(t *testing.T) {
	err := validateContainerDeployVersion("v2", "9b8a7c6")
	if err == nil || !strings.Contains(err.Error(), "use either --tag or --commit-id, not both") {
		t.Fatalf("expected the flags to be rejected together, got %v", err)
	}

	if err := validateContainerDeployVersion("v2", ""); err != nil {
		t.Fatalf("expected --tag alone to be accepted, got %v", err)
	}
	if err := validateContainerDeployVersion("", "9b8a7c6"); err != nil {
		t.Fatalf("expected --commit-id alone to be accepted, got %v", err)
	}
}

func TestContainerDeployWithoutVersionSendsNoTagForGitContainer(t *testing.T) {
	client, requests := newRecordingQoveryClient(t)
	imageContainer := containerWithSource(t, "container-image", "web", imageContainerSourceJson)
	olderApiContainer := containerWithSource(t, "container-older-api", "worker", "")
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)

	err := deployContainers(client, "env-1", []*qovery.ContainerResponse{imageContainer, olderApiContainer, gitContainer}, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(*requests) != 1 || (*requests)[0].path != "/environment/env-1/service/deploy" {
		t.Fatalf("expected one deploy-all request, got %v", *requests)
	}
	var body qovery.DeployAllRequest
	if err := json.Unmarshal([]byte((*requests)[0].body), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Containers) != 3 {
		t.Fatalf("expected three containers, got %v", body.Containers)
	}
	if body.Containers[0].GetImageTag() != "1.27" || body.Containers[1].GetImageTag() != "1.27" {
		t.Errorf("expected image containers to keep their running tag, got %v", body.Containers)
	}
	if body.Containers[2].ImageTag != nil {
		t.Errorf("expected no image tag for the Git container, got %s", *body.Containers[2].ImageTag)
	}
}

func TestContainerDeployErrorKeepsTheApiMessage(t *testing.T) {
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)
	imageContainer := containerWithSource(t, "container-image", "web", imageContainerSourceJson)
	cases := []struct {
		name       string
		containers []*qovery.ContainerResponse
		tag        string
		commitId   string
	}{
		{name: "commit on a Git container", containers: []*qovery.ContainerResponse{gitContainer}, commitId: "9b8a7c6"},
		{name: "tag on an image container", containers: []*qovery.ContainerResponse{imageContainer}, tag: "v2"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, _ := newRecordingQoveryClientRespondingWith(t, http.StatusBadRequest,
				`{"status":400,"detail":"the requested version does not exist"}`)

			err := deployContainers(client, "env-1", c.containers, c.tag, c.commitId)
			if err == nil || !strings.Contains(err.Error(), "the requested version does not exist") {
				t.Fatalf("expected the API message in the error, got %v", err)
			}
		})
	}
}

func TestContainerDeployChecksSourceBeforeWatching(t *testing.T) {
	gitContainer := containerWithSource(t, "container-git", "api", gitContainerSourceJson)
	imageContainer := containerWithSource(t, "container-image", "web", imageContainerSourceJson)
	cases := []struct {
		name          string
		containers    []*qovery.ContainerResponse
		tag           string
		commitId      string
		expectedError string
	}{
		{name: "tag on a Git container", containers: []*qovery.ContainerResponse{imageContainer, gitContainer}, tag: "v2", expectedError: "api is a Git container"},
		{name: "commit on an image container", containers: []*qovery.ContainerResponse{gitContainer, imageContainer}, commitId: "9b8a7c6", expectedError: "--commit-id only applies to Git containers"},
		{name: "tag on an image container", containers: []*qovery.ContainerResponse{imageContainer}, tag: "v2"},
		{name: "commit on a Git container", containers: []*qovery.ContainerResponse{gitContainer}, commitId: "9b8a7c6"},
		{name: "no version", containers: []*qovery.ContainerResponse{imageContainer, gitContainer}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkContainersMatchDeployVersion(c.containers, c.tag, c.commitId)
			if c.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.expectedError) {
				t.Fatalf("expected %q in the error, got %v", c.expectedError, err)
			}
		})
	}
}
