package cmd

import (
	"encoding/json"
	"testing"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

func setFlagForTest(t *testing.T, cmd *cobra.Command, name string, value string) {
	t.Helper()
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		t.Fatalf("flag --%s is not registered on %s", name, cmd.CommandPath())
	}
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
}

func requestBody(t *testing.T, request interface{}) map[string]interface{} {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func storedContainerRequest(autoDeploy *bool, autoPreview bool) qovery.ContainerRequest {
	return qovery.ContainerRequest{
		Name:        "api",
		AutoPreview: utils.Bool(autoPreview),
		AutoDeploy:  *qovery.NewNullableBool(autoDeploy),
	}
}

func TestContainerUpdateSendsAutoDeployAndAutoPreviewWhenGiven(t *testing.T) {
	setFlagForTest(t, containerUpdateCmd, "auto-deploy", "true")
	setFlagForTest(t, containerUpdateCmd, "auto-preview", "true")
	req := storedContainerRequest(nil, false)

	applyContainerAutoFlags(containerUpdateCmd, &req)

	body := requestBody(t, req)
	if body["auto_deploy"] != true || body["auto_preview"] != true {
		t.Fatalf("expected auto_deploy and auto_preview true, got %v", body)
	}
}

func TestContainerUpdateTurnsAutoDeployAndAutoPreviewOff(t *testing.T) {
	setFlagForTest(t, containerUpdateCmd, "auto-deploy", "false")
	setFlagForTest(t, containerUpdateCmd, "auto-preview", "false")
	req := storedContainerRequest(utils.Bool(true), true)

	applyContainerAutoFlags(containerUpdateCmd, &req)

	body := requestBody(t, req)
	if body["auto_deploy"] != false || body["auto_preview"] != false {
		t.Fatalf("expected auto_deploy and auto_preview false, got %v", body)
	}
}

func TestContainerUpdateKeepsStoredAutoDeployAndAutoPreviewWhenNotGiven(t *testing.T) {
	for _, stored := range []struct {
		autoDeploy  *bool
		autoPreview bool
	}{
		{autoDeploy: utils.Bool(true), autoPreview: true},
		{autoDeploy: utils.Bool(false), autoPreview: false},
		{autoDeploy: nil, autoPreview: true},
	} {
		req := storedContainerRequest(stored.autoDeploy, stored.autoPreview)
		expected := requestBody(t, req)

		applyContainerAutoFlags(containerUpdateCmd, &req)

		body := requestBody(t, req)
		if body["auto_deploy"] != expected["auto_deploy"] || body["auto_preview"] != expected["auto_preview"] {
			t.Fatalf("expected stored auto_deploy %v and auto_preview %v, got %v", expected["auto_deploy"], expected["auto_preview"], body)
		}
		if _, sent := body["auto_deploy"]; !sent {
			t.Fatalf("expected auto_deploy to be sent, got %v", body)
		}
	}
}
