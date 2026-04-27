package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCmdFlags(t *testing.T) {
	flags := []string{"organization", "project", "environment", "application", "container", "database", "job", "service", "raw"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, logCmd.Flags().Lookup(name), "flag --%s should be registered", name)
		})
	}
}

func TestLogCmdUnknownFlag(t *testing.T) {
	err := logCmd.ParseFlags([]string{"--unknown-flag", "value"})
	assert.Error(t, err)
}

func TestLogCmdFlagParsing(t *testing.T) {
	// Reset flags before parsing
	_ = logCmd.Flags().Set("container", "")
	_ = logCmd.Flags().Set("project", "")
	_ = logCmd.Flags().Set("environment", "")

	err := logCmd.ParseFlags([]string{"--container", "test", "--project", "Laura", "--environment", "keda"})
	require.NoError(t, err)

	got, err := logCmd.Flags().GetString("container")
	require.NoError(t, err)
	assert.Equal(t, "test", got)

	got, err = logCmd.Flags().GetString("project")
	require.NoError(t, err)
	assert.Equal(t, "Laura", got)

	got, err = logCmd.Flags().GetString("environment")
	require.NoError(t, err)
	assert.Equal(t, "keda", got)
}
