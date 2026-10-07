package cmd

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateStateBackend(t *testing.T) {
	assert.NoError(t, validateStateBackend("s3"))
	assert.NoError(t, validateStateBackend("qovery"))
	assert.ErrorContains(t, validateStateBackend(""), "--backend is required")
	assert.ErrorContains(t, validateStateBackend("S3"), "invalid --backend")
	assert.ErrorContains(t, validateStateBackend("kubernetes"), "invalid --backend")
}

func TestBuildStateBackendRequestGet(t *testing.T) {
	path, body, err := buildStateBackendRequest(http.MethodGet, "bp-1", "")

	assert.NoError(t, err)
	assert.Equal(t, "blueprint/bp-1/stateBackend", path)
	assert.Nil(t, body)
}

func TestBuildStateBackendRequestPut(t *testing.T) {
	path, body, err := buildStateBackendRequest(http.MethodPut, "bp-1", "s3")

	assert.NoError(t, err)
	assert.Equal(t, "blueprint/bp-1/stateBackend", path)
	payload, _ := io.ReadAll(body)
	assert.JSONEq(t, `{"backend":"s3"}`, string(payload))
}

func stateBackendHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestParseStateBackendResponse(t *testing.T) {
	result, err := parseStateBackendResponse(stateBackendHTTPResponse(200, `{"backend":"s3","state_migration_pending":true}`))

	assert.NoError(t, err)
	assert.Equal(t, "s3", result.Backend)
	assert.True(t, result.StateMigrationPending)
}

func TestParseStateBackendResponseErrorDetail(t *testing.T) {
	_, err := parseStateBackendResponse(stateBackendHTTPResponse(422, `{"status":422,"detail":"S3 backend requires an AWS cluster"}`))

	assert.EqualError(t, err, "S3 backend requires an AWS cluster")
}

func TestParseStateBackendResponseErrorWithoutDetail(t *testing.T) {
	_, err := parseStateBackendResponse(stateBackendHTTPResponse(500, `boom`))

	assert.EqualError(t, err, "unexpected status 500: boom")
}

func TestStateBackendSwitchWarningMigrates(t *testing.T) {
	current := stateBackendResponse{Backend: "qovery"}

	assert.Equal(t, "The next deployment migrates the Terraform state from qovery to s3.", stateBackendSwitchWarning(current, "s3"))
}

func TestStateBackendSwitchWarningCancelsPendingMigration(t *testing.T) {
	current := stateBackendResponse{Backend: "s3", StateMigrationPending: true}

	assert.Equal(t, "This switch cancels the pending migration; the Terraform state stays in qovery.", stateBackendSwitchWarning(current, "qovery"))
}

func TestStateBackendUnchangedMessage(t *testing.T) {
	assert.Equal(t, "Blueprint already uses the s3 state backend", stateBackendUnchangedMessage(stateBackendResponse{Backend: "s3"}))
	assert.Equal(t, "Blueprint already uses the s3 state backend; the state migration from qovery is pending",
		stateBackendUnchangedMessage(stateBackendResponse{Backend: "s3", StateMigrationPending: true}))
}

func TestStateBackendDeployFailureSaysBackendWasSwitched(t *testing.T) {
	err := stateBackendDeployFailure(stateBackendResponse{Backend: "s3", StateMigrationPending: true}, errors.New("503"))

	assert.EqualError(t, err, "state backend is switched to s3 (migration pending) but the deployment could not be triggered: 503; deploy the blueprint to migrate the state")
}

func noBlueprintLookup(t *testing.T) func(string) (string, bool, error) {
	return func(id string) (string, bool, error) {
		t.Fatalf("unexpected blueprint lookup for %s", id)
		return "", false, nil
	}
}

func TestMatchBlueprintIdPrefersServiceNameWithoutFetching(t *testing.T) {
	candidates := []blueprintCandidate{{blueprintId: "bp-1", serviceName: "other"}, {blueprintId: "bp-2", serviceName: "rabbit"}}

	id, err := matchBlueprintId(candidates, "rabbit", noBlueprintLookup(t))

	assert.NoError(t, err)
	assert.Equal(t, "bp-2", id)
}

func TestMatchBlueprintIdSkipsInaccessibleBlueprints(t *testing.T) {
	candidates := []blueprintCandidate{{blueprintId: "bp-1", serviceName: "svc-1"}, {blueprintId: "bp-2", serviceName: "svc-2"}}
	names := map[string]string{"bp-2": "rabbit"}

	id, err := matchBlueprintId(candidates, "rabbit", func(id string) (string, bool, error) {
		name, found := names[id]
		return name, found, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "bp-2", id)
}

func TestMatchBlueprintIdAmbiguous(t *testing.T) {
	candidates := []blueprintCandidate{{blueprintId: "bp-1", serviceName: "rabbit"}, {blueprintId: "bp-2", serviceName: "rabbit"}}

	_, err := matchBlueprintId(candidates, "rabbit", noBlueprintLookup(t))

	assert.EqualError(t, err, "several blueprints match rabbit, use --id instead")
}

func TestMatchBlueprintIdNotFound(t *testing.T) {
	_, err := matchBlueprintId([]blueprintCandidate{{blueprintId: "bp-1", serviceName: "svc"}}, "rabbit",
		func(string) (string, bool, error) { return "other", true, nil })

	assert.EqualError(t, err, "blueprint rabbit not found in the environment")
}

func TestBlueprintStateBackendIdAndNameMutuallyExclusive(t *testing.T) {
	cmd := blueprintStateBackendSetCmd
	defer func() {
		_ = cmd.Flags().Set("id", "")
		_ = cmd.Flags().Set("blueprint", "")
		blueprintId, blueprintName = "", ""
		cmd.Flags().Lookup("id").Changed = false
		cmd.Flags().Lookup("blueprint").Changed = false
	}()

	assert.NoError(t, cmd.Flags().Set("id", "bp-1"))
	assert.NoError(t, cmd.Flags().Set("blueprint", "rabbit"))

	assert.ErrorContains(t, cmd.ValidateFlagGroups(), "none of the others can be")
}

func TestBlueprintParentCommandsRejectUnknownArgs(t *testing.T) {
	assert.Error(t, blueprintCmd.Args(blueprintCmd, []string{"stat-backend"}))
	assert.Error(t, blueprintStateBackendCmd.Args(blueprintStateBackendCmd, []string{"gett"}))
}
