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

	if err == nil || err.Error() != "major 18 is not available: the database is already on the latest major" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfirmationTextPerMode(t *testing.T) {
	cases := map[qovery.BlueprintMajorSwitchMode]string{
		qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL: "The database already runs 18: only Qovery's records change, the database is not touched.",
		qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE: "This upgrades the database from 14 to 18 in place, with downtime. Review the preview in the Console first if needed.",
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
