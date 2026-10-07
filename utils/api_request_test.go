package utils

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPIRequestSetsBaseURLAuthAndUserAgent(t *testing.T) {
	t.Setenv("QOVERY_API_URL", "http://localhost:8080/")
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")

	req, err := NewAPIRequest("GET", "/blueprint/bp-1/stateBackend", nil, true)

	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/blueprint/bp-1/stateBackend", req.URL.String())
	assert.Equal(t, "CLI "+Version, req.Header.Get("User-Agent"))
	assert.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestDumpRequestRedactedHidesToken(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "super-secret-token")
	req, err := NewAPIRequest("PUT", "blueprint/bp-1/stateBackend", strings.NewReader(`{"backend":"s3"}`), true)
	assert.NoError(t, err)

	dump, err := dumpRequestRedacted(req)

	assert.NoError(t, err)
	assert.NotContains(t, dump, "super-secret-token")
	assert.Regexp(t, `Authorization: \w+ \*\*\*`, dump)
	assert.Contains(t, dump, `{"backend":"s3"}`)
	assert.Contains(t, req.Header.Get("Authorization"), "super-secret-token")
	body, _ := io.ReadAll(req.Body)
	assert.Equal(t, `{"backend":"s3"}`, string(body))
}

func TestRedactAuthorization(t *testing.T) {
	assert.Equal(t, "Bearer ***", redactAuthorization("Bearer abc"))
	assert.Equal(t, "Token ***", redactAuthorization("Token abc"))
	assert.Equal(t, "***", redactAuthorization("abc"))
}
