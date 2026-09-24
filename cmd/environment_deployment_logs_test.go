package cmd

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentDeploymentLogsCmdFlags(t *testing.T) {
	flags := []string{"organization", "project", "environment", "execution-id", "service", "service-id", "pre-check", "errors-only", "json"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, environmentDeploymentLogsCmd.Flags().Lookup(name), "flag --%s should be registered", name)
		})
	}
}

func TestEnvironmentDeploymentLogsCmdIsRegisteredUnderDeployment(t *testing.T) {
	for _, c := range environmentDeploymentCmd.Commands() {
		if c.Name() == "logs" {
			return
		}
	}
	t.Fatal("`logs` should be registered under `environment deployment`")
}

func TestEnvironmentDeploymentLogsCmdUnknownFlag(t *testing.T) {
	restoreDeploymentLogsFlags(t)

	err := environmentDeploymentLogsCmd.ParseFlags([]string{"--unknown-flag", "value"})
	assert.Error(t, err)
}

func TestEnvironmentDeploymentLogsCmdFlagParsing(t *testing.T) {
	// Cobra binds these flags to package-level vars that environmentDeploymentLogsCmd.Run
	// reads, so parsing here would otherwise leave values behind for every later test in
	// this package -- and make failures depend on test order.
	restoreDeploymentLogsFlags(t)

	// `explain` binds the shared package-level `id` var to its own --id. This command accepts
	// --id as an alias for --execution-id (see TestEnvironmentDeploymentLogsCmdAcceptsIdAsAlias...),
	// but must write to its own var, so the two commands cannot clobber each other.
	//
	// Snapshot rather than assert emptiness: another test in this package may legitimately
	// have parsed `explain --id` first. What matters is that parsing ours does not touch it.
	idBefore := id

	err := environmentDeploymentLogsCmd.ParseFlags([]string{
		"--execution-id", "exec-123", "--service", "my-app", "--errors-only",
	})
	require.NoError(t, err)

	assert.Equal(t, "exec-123", deploymentLogsExecutionId)
	assert.Equal(t, "my-app", deploymentLogsServiceName)
	assert.True(t, deploymentLogsErrorsOnly)
	assert.Equal(t, idBefore, id, "parsing --execution-id must leave the shared `id` var alone")
}

// restoreDeploymentLogsFlags captures the flag-backed globals and restores them, and the
// flag values cobra holds, when the test finishes.
func restoreDeploymentLogsFlags(t *testing.T) {
	t.Helper()

	executionId, serviceName, serviceId := deploymentLogsExecutionId, deploymentLogsServiceName, deploymentLogsServiceId
	preCheck, errorsOnly := deploymentLogsPreCheck, deploymentLogsErrorsOnly

	t.Cleanup(func() {
		deploymentLogsExecutionId, deploymentLogsServiceName, deploymentLogsServiceId = executionId, serviceName, serviceId
		deploymentLogsPreCheck, deploymentLogsErrorsOnly = preCheck, errorsOnly

		// The vars above are cobra's destinations; the flag objects keep their own copy of
		// the value and of Changed, so reset those too or a later ParseFlags starts dirty.
		for name, value := range map[string]string{
			"execution-id": executionId,
			"service":      serviceName,
			"service-id":   serviceId,
			"pre-check":    strconv.FormatBool(preCheck),
			"errors-only":  strconv.FormatBool(errorsOnly),
		} {
			f := environmentDeploymentLogsCmd.Flags().Lookup(name)
			if f == nil {
				continue
			}
			_ = f.Value.Set(value)
			f.Changed = false
		}
	})
}

func TestEnvironmentDeploymentLogsCmdAcceptsIdAsAliasForExecutionId(t *testing.T) {
	// `deployment list` prints this id and `deployment explain` takes it as --id, so the
	// same value must be copyable into this command under either spelling.
	restoreDeploymentLogsFlags(t)

	require.NoError(t, environmentDeploymentLogsCmd.ParseFlags([]string{"--id", "env-7"}))
	assert.Equal(t, "env-7", deploymentLogsExecutionId, "--id should set the execution id")

	require.NoError(t, environmentDeploymentLogsCmd.ParseFlags([]string{"--execution-id", "env-9"}))
	assert.Equal(t, "env-9", deploymentLogsExecutionId, "--execution-id should set the same var")

	// One flag with two spellings, not two flags: help must not list --id separately, and
	// the alias must resolve to the same flag object.
	assert.Nil(t, environmentDeploymentLogsCmd.Flags().ShorthandLookup("i"))
	assert.Same(t,
		environmentDeploymentLogsCmd.Flags().Lookup("execution-id"),
		environmentDeploymentLogsCmd.Flags().Lookup("id"),
		"--id must resolve to the --execution-id flag, not a second one")

	// The alias must not reach the shared `id` var that `explain` binds to --id.
	idBefore := id
	require.NoError(t, environmentDeploymentLogsCmd.ParseFlags([]string{"--id", "env-11"}))
	assert.Equal(t, idBefore, id, "--id here must not write to the shared `id` var")
}
