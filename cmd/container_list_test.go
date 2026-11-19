package cmd

import (
	"encoding/json"
	"testing"

	"github.com/qovery/qovery-client-go"
)

func TestContainerListShowsImageAndGitSources(t *testing.T) {
	containers := []qovery.ContainerResponse{
		*containerWithSource(t, "container-image", "web", imageContainerSourceJson),
		*containerWithSource(t, "container-git", "api", gitContainerSourceJson),
	}
	statuses := &qovery.EnvironmentStatuses{}

	rows := getContainerTableRows(containers, statuses)
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %v", rows)
	}
	if source, image := rows[0][5], rows[0][6]; source != "Registry Docker Hub" || image != "nginx:1.27" {
		t.Errorf("unexpected image container source %q and image %q", source, image)
	}
	if source, image := rows[1][5], rows[1][6]; source != "Git https://github.com/acme/api.git (branch main)" || image != "z1234-api:4f2c1e0" {
		t.Errorf("unexpected Git container source %q and image %q", source, image)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(getContainerJsonOutput(containers, statuses)), &results); err != nil {
		t.Fatal(err)
	}
	if results[0]["source"] != "image" || results[0]["registry"] != "Docker Hub" || results[0]["image_name"] != "nginx" || results[0]["git_repository"] != nil {
		t.Errorf("unexpected image container JSON %v", results[0])
	}
	repository, _ := results[1]["git_repository"].(map[string]interface{})
	if results[1]["source"] != "git_repository" || repository["url"] != "https://github.com/acme/api.git" || repository["branch"] != "main" || results[1]["tag"] != "4f2c1e0" {
		t.Errorf("unexpected Git container JSON %v", results[1])
	}
}
