package cmd

import (
	"encoding/json"
	"errors"
	"testing"

	qovery "github.com/qovery/qovery-client-go"
)

func newMajor(version string, mode qovery.BlueprintMajorSwitchMode) qovery.BlueprintUpdateNewMajorVersion {
	return *qovery.NewBlueprintUpdateNewMajorVersion(version, "AWS/postgres/"+version+"/1.5.0", mode)
}

func TestPickMajorReturnsTheRequestedMajor(t *testing.T) {
	majors := []qovery.BlueprintUpdateNewMajorVersion{
		newMajor("17", qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE),
		newMajor("18", qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL),
	}

	picked, err := pickMajor(majors, "18")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked.GetLatestTag() != "AWS/postgres/18/1.5.0" || picked.GetMode() != qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL {
		t.Fatalf("picked the wrong major: %+v", picked)
	}
}

func TestPickMajorListsTheAvailableMajorsWhenMissing(t *testing.T) {
	majors := []qovery.BlueprintUpdateNewMajorVersion{
		newMajor("17", qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE),
		newMajor("18", qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE),
	}

	_, err := pickMajor(majors, "19")

	if err == nil || err.Error() != "major 19 is not available, choose one of: 17, 18" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPickMajorSaysWhenAlreadyOnTheLatestMajor(t *testing.T) {
	_, err := pickMajor(nil, "18")

	if err == nil || err.Error() != "major 18 is not available: the blueprint is already on the latest major" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfirmationTextPerMode(t *testing.T) {
	cases := map[qovery.BlueprintMajorSwitchMode]string{
		qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL: "The service already runs 18: only Qovery's records change, the service is not touched.",
		qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE: "This upgrades the service from 14 to 18 in place on the next deploy (downtime for databases). Review the preview in the Console first if needed.",
	}
	for mode, expected := range cases {
		if got := confirmationText(mode, "14", "18"); got != expected {
			t.Errorf("mode %s: got %q, want %q", mode, got, expected)
		}
	}
}

func TestUpgradeRequestLeavesVariablesAndOverridesUnchanged(t *testing.T) {
	request := buildUpdateRequest("orders-db", "AWS/postgres/18/1.5.0", "app://qovery-console/postgresql")

	if request.Name != "orders-db" || request.Tag != "AWS/postgres/18/1.5.0" || request.Icon != "app://qovery-console/postgresql" {
		t.Fatalf("unexpected request: %+v", request)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(body); got != `{"icon":"app://qovery-console/postgresql","name":"orders-db","tag":"AWS/postgres/18/1.5.0"}` {
		t.Fatalf("variables and spec overrides must stay out of the request, got %s", got)
	}
}

func TestMajorOfTag(t *testing.T) {
	if got := majorOfTag("AWS/postgres/14/4.6.0"); got != "14" {
		t.Fatalf("got %q", got)
	}
	if got := majorOfTag("weird"); got != "weird" {
		t.Fatalf("got %q", got)
	}
}

func TestUpgradeApiErrorKeepsPlainErrors(t *testing.T) {
	plain := errors.New("boom")
	if apiError(plain) != plain {
		t.Fatal("a non-API error must be returned as is")
	}
	if apiError(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

func TestPickBlueprintServiceFindsTerraformAndHelmBlueprints(t *testing.T) {
	services := []blueprintService{
		{serviceId: "tf-1", name: "orders-db", blueprintId: "bp-1", iconUri: "app://qovery-console/postgresql"},
		{serviceId: "helm-1", name: "cache", blueprintId: "bp-2", iconUri: "app://qovery-console/redis"},
	}

	for _, name := range []string{"orders-db", "cache"} {
		service, err := pickBlueprintService(services, name)
		if err != nil || service.name != name {
			t.Fatalf("%s: got %+v, %v", name, service, err)
		}
	}
}

func TestPickBlueprintServiceRejectsAServiceWithoutBlueprint(t *testing.T) {
	services := []blueprintService{{serviceId: "tf-1", name: "plain-terraform"}}

	_, err := pickBlueprintService(services, "plain-terraform")

	if err == nil || err.Error() != "plain-terraform is not a blueprint service" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPickBlueprintServiceReportsAnUnknownName(t *testing.T) {
	_, err := pickBlueprintService(nil, "missing")

	if err == nil || err.Error() != "no blueprint service named missing in this environment" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefusedSwitchLetsOnlyRelabelAndUpgradeThrough(t *testing.T) {
	for _, mode := range []qovery.BlueprintMajorSwitchMode{qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL, qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE} {
		if err := refusedSwitch("db", newMajor("18", mode)); err != nil {
			t.Fatalf("mode %s: unexpected error: %v", mode, err)
		}
	}

	unsupported := newMajor("13", qovery.BLUEPRINTMAJORSWITCHMODE_UNSUPPORTED)
	unsupported.SetUnsupportedReason("the target major is lower than the version running")
	if err := refusedSwitch("db", unsupported); err == nil || err.Error() != "cannot move db to major 13: the target major is lower than the version running" {
		t.Fatalf("unsupported mode: got %v", err)
	}

	for _, mode := range []qovery.BlueprintMajorSwitchMode{"", "SOMETHING_NEW"} {
		if err := refusedSwitch("db", newMajor("18", mode)); err == nil {
			t.Fatalf("mode %q must be refused", mode)
		}
	}
}

func TestBlueprintCommandRejectsUnknownArguments(t *testing.T) {
	if err := blueprintCmd.Args(blueprintCmd, []string{"bogus"}); err == nil {
		t.Fatal("expected an error for an unknown argument")
	}
}
