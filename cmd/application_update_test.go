package cmd

import (
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
