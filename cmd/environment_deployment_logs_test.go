package cmd

import (
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
	err := environmentDeploymentLogsCmd.ParseFlags([]string{"--unknown-flag", "value"})
	assert.Error(t, err)
}

func TestEnvironmentDeploymentLogsCmdFlagParsing(t *testing.T) {
	// `id` is bound to --id on the sibling `explain` command; the logs command must carry
	// its own execution id so the two do not clobber each other.
	require.Nil(t, environmentDeploymentLogsCmd.Flags().Lookup("id"), "--id should not be registered on the logs command")

	err := environmentDeploymentLogsCmd.ParseFlags([]string{
		"--execution-id", "exec-123", "--service", "my-app", "--errors-only",
	})
	require.NoError(t, err)

	assert.Equal(t, "exec-123", deploymentLogsExecutionId)
	assert.Equal(t, "my-app", deploymentLogsServiceName)
	assert.True(t, deploymentLogsErrorsOnly)
	assert.Empty(t, id, "parsing --execution-id must leave the shared `id` var alone")
}
