package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUsingEnvToken_WithAPIToken(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "qov_test_token_abc123")
	assert.True(t, IsUsingEnvToken())
}

func TestIsUsingEnvToken_WithJWTInEnvVar(t *testing.T) {
	// Even JWTs in env vars are non-refreshable: GetAccessToken always
	// returns the env var directly, ignoring the stored context.
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.signature")
	assert.True(t, IsUsingEnvToken())
}

func TestIsUsingEnvToken_WithLegacyEnvVar(t *testing.T) {
	t.Setenv("Q_CLI_ACCESS_TOKEN", "qov_test_token_abc123")
	assert.True(t, IsUsingEnvToken())
}

func TestIsUsingEnvToken_WithoutEnvVar(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "")
	t.Setenv("Q_CLI_ACCESS_TOKEN", "")
	assert.False(t, IsUsingEnvToken())
}

func TestForceRefreshAccessToken_EmptyRefreshToken(t *testing.T) {
	// Point HOME to a temp dir and create a minimal context with no refresh token
	t.Setenv("HOME", t.TempDir())
	err := InitializeQoveryContext()
	assert.NoError(t, err)

	_, err = ForceRefreshAccessToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no refresh token available")
}
