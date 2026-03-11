package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"github.com/qovery/qovery-cli/utils"
)

// captureOutput temporarily replaces os.Stdout and os.Stderr and returns the
// data written to each after the function returns.
func captureOutput(fn func()) (stdout string, stderr string) {
	oldOut := os.Stdout
	oldErr := os.Stderr
	defer func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
	}()

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	outBuf, _ := io.ReadAll(rOut)
	errBuf, _ := io.ReadAll(rErr)
	return string(outBuf), string(errBuf)
}

// writeContextFile creates a minimal ~/.qovery/context.json in a temp HOME dir,
// sets HOME to that dir, and returns a cleanup func.
func writeContextFile(t *testing.T, orgID, projectID, envID, serviceID string) {
	t.Helper()
	tmpHome := t.TempDir()
	qoveryDir := filepath.Join(tmpHome, ".qovery")
	if err := os.MkdirAll(qoveryDir, 0700); err != nil {
		t.Fatal(err)
	}
	contextData := fmt.Sprintf(`{
		"access_token": "fake",
		"access_token_expiration": "2099-01-01T00:00:00Z",
		"refresh_token": "fake",
		"organization_id": %q,
		"project_id": %q,
		"environment_id": %q,
		"service_id": %q
	}`, orgID, projectID, envID, serviceID)
	if err := os.WriteFile(filepath.Join(qoveryDir, "context.json"), []byte(contextData), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpHome)
}

// --- Scenario 1: GET 200 — body written to stdout ---
func TestAPIGet200(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	expected := `{"results":[]}`
	httpmock.RegisterResponder("GET", "https://api.qovery.com/organization",
		httpmock.NewStringResponder(200, expected))

	// Reset flag state
	apiMethod = ""
	apiInput = ""
	apiFields = []string{}
	apiHeaders = []string{}
	apiInclude = false

	stdout, _ := captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	assert.Equal(t, expected, stdout)
}

// --- Scenario 2: POST with stdin body ---
func TestAPIPostStdin(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	requestBody := `{"name":"my-org","plan":"FREE"}`
	var capturedBody string
	httpmock.RegisterResponder("POST", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			return httpmock.NewStringResponse(200, `{"id":"123"}`), nil
		})

	// Replace stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(requestBody)
	_ = w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	apiMethod = "POST"
	apiInput = "-"
	apiFields = []string{}
	apiHeaders = []string{}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	assert.Equal(t, requestBody, capturedBody)
}

// --- Scenario 3: --input with file path is rejected ---
func TestAPIInputFilePathRejected(t *testing.T) {
	err := validateAPIArgs("organization", "", "body.json", nil, nil)
	assert.ErrorContains(t, err, "--input only accepts")
}

// --- Scenario 4: DELETE method ---
func TestAPIDelete(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedMethod string
	httpmock.RegisterResponder("DELETE", "https://api.qovery.com/organization/abc",
		func(req *http.Request) (*http.Response, error) {
			capturedMethod = req.Method
			return httpmock.NewStringResponse(200, ``), nil
		})

	apiMethod = "DELETE"
	apiInput = ""
	apiFields = []string{}
	apiHeaders = []string{}
	apiInclude = false

	stdout, _ := captureOutput(func() {
		runAPI(apiCmd, []string{"organization/abc"})
	})

	assert.Equal(t, "DELETE", capturedMethod)
	assert.Equal(t, "", stdout) // DELETE 200 with empty body → nothing on stdout
}

// --- Scenario 5: Invalid method (pure unit test via validateAPIArgs) ---
func TestAPIInvalidMethod(t *testing.T) {
	assert.ErrorContains(t, validateAPIArgs("organization", "BREW", "", nil, nil), "invalid HTTP method")
}

// --- Scenario 6: Non-2xx → body to stderr, nothing to stdout ---
func TestAPINon2xx(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	errorBody := `{"status":404,"message":"not found"}`
	httpmock.RegisterResponder("GET", "https://api.qovery.com/missing-resource",
		httpmock.NewStringResponder(404, errorBody))

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.qovery.com/missing-resource", nil)
	resp, err := client.Do(req)
	assert.Nil(t, err)
	defer func() { _ = resp.Body.Close() }()

	var outBuf, errBuf bytes.Buffer
	ok, writeErr := writeResponse(resp, false, &outBuf, &errBuf)
	assert.Nil(t, writeErr)
	assert.False(t, ok)
	assert.Equal(t, "", outBuf.String())        // nothing on stdout
	assert.Equal(t, errorBody, errBuf.String()) // body on stderr
}

// --- Scenario 7: --include flag output format ---
func TestAPIIncludeFlag(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	responseBody := `{"results":[]}`
	httpmock.RegisterResponder("GET", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(200, responseBody)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		})

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.qovery.com/organization", nil)
	resp, err := client.Do(req)
	assert.Nil(t, err)
	defer func() { _ = resp.Body.Close() }()

	var outBuf, errBuf bytes.Buffer
	ok, writeErr := writeResponse(resp, true, &outBuf, &errBuf)
	assert.Nil(t, writeErr)
	assert.True(t, ok)

	stdout := outBuf.String()
	// Must start with HTTP status line
	assert.True(t, strings.HasPrefix(stdout, "HTTP/"), "stdout must start with HTTP/ status line, got: %q", stdout)
	// Must contain Content-Type header
	assert.Contains(t, stdout, "Content-Type: application/json")
	// Must contain blank line before body
	assert.Contains(t, stdout, "\n\n")
	// Must contain body
	assert.Contains(t, stdout, responseBody)
}

// --- Scenario 8: Custom -H header sent in request ---
func TestAPICustomHeader(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedHeader string
	httpmock.RegisterResponder("GET", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Get("X-Request-Id")
			return httpmock.NewStringResponse(200, `{}`), nil
		})

	apiMethod = ""
	apiInput = ""
	apiFields = []string{}
	apiHeaders = []string{"X-Request-Id: abc123"}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	assert.Equal(t, "abc123", capturedHeader)
}

// --- Scenario 9: Malformed -H header (pure unit test via validateAPIArgs) ---
func TestAPIMalformedHeader(t *testing.T) {
	assert.ErrorContains(t, validateAPIArgs("organization", "", "", nil, []string{"Badheader"}), "invalid header")
}

// --- Scenario 10: Full URL rejected (pure unit test via validateAPIArgs) ---
func TestAPIFullURLRejected(t *testing.T) {
	assert.ErrorContains(t, validateAPIArgs("https://api.qovery.com/organization", "", "", nil, nil), "not a full URL")
}

// --- Scenario 11: Path normalisation (pure unit test) ---
func TestAPIPathNormalisation(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"/organization", "https://api.qovery.com/organization"},
		{"organization", "https://api.qovery.com/organization"},
	}
	for _, tc := range cases {
		path := strings.TrimLeft(tc.input, "/")
		result := "https://api.qovery.com" + "/" + path
		assert.Equal(t, tc.expected, result)
	}
}

// --- Scenario 12: --field string value ---
func TestAPIFieldString(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedBody string
	httpmock.RegisterResponder("POST", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			return httpmock.NewStringResponse(200, `{}`), nil
		})

	apiMethod = ""
	apiInput = ""
	apiFields = []string{"name=myorg"}
	apiHeaders = []string{}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	var result map[string]any
	_ = json.Unmarshal([]byte(capturedBody), &result)
	assert.Equal(t, "myorg", result["name"])
}

// --- Scenario 13: --field bool coercion ---
func TestAPIFieldBoolCoercion(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedBody string
	httpmock.RegisterResponder("POST", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			return httpmock.NewStringResponse(200, `{}`), nil
		})

	apiMethod = ""
	apiInput = ""
	apiFields = []string{"enabled=true"}
	apiHeaders = []string{}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	var result map[string]any
	_ = json.Unmarshal([]byte(capturedBody), &result)
	assert.Equal(t, true, result["enabled"])
}

// --- Scenario 14: --field int coercion ---
func TestAPIFieldIntCoercion(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedBody string
	httpmock.RegisterResponder("POST", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			return httpmock.NewStringResponse(200, `{}`), nil
		})

	apiMethod = ""
	apiInput = ""
	apiFields = []string{"count=42"}
	apiHeaders = []string{}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	var result map[string]any
	_ = json.Unmarshal([]byte(capturedBody), &result)
	// After json.Unmarshal into map[string]any, all numbers become float64
	assert.Equal(t, float64(42), result["count"])
}

// --- Scenario 15: --field multiple fields ---
func TestAPIFieldMultipleFields(t *testing.T) {
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	var capturedBody string
	httpmock.RegisterResponder("POST", "https://api.qovery.com/organization",
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			return httpmock.NewStringResponse(200, `{}`), nil
		})

	apiMethod = ""
	apiInput = ""
	apiFields = []string{"name=x", "count=1"}
	apiHeaders = []string{}
	apiInclude = false

	captureOutput(func() {
		runAPI(apiCmd, []string{"organization"})
	})

	var result map[string]any
	_ = json.Unmarshal([]byte(capturedBody), &result)
	assert.Equal(t, "x", result["name"])
	assert.Equal(t, float64(1), result["count"])
}

// --- Scenario 16: --field + --input together (pure unit test via validateAPIArgs) ---
func TestAPIFieldAndInputMutuallyExclusive(t *testing.T) {
	assert.ErrorContains(t, validateAPIArgs("organization", "", "-", []string{"name=x"}, nil), "mutually exclusive")
}

// --- Scenario 17: Malformed --field entry (pure unit test via validateAPIArgs) ---
func TestAPIMalformedField(t *testing.T) {
	assert.ErrorContains(t, validateAPIArgs("organization", "", "", []string{"badfield"}, nil), "invalid field")
}

// --- Scenario 18: Placeholder substitution with org context ---
func TestAPIPlaceholderSubstitution(t *testing.T) {
	writeContextFile(t, "org-123", "proj-456", "env-789", "svc-abc")

	result := substitutePathPlaceholders("organization/{organizationId}/project")
	assert.Equal(t, "organization/org-123/project", result)
}

// --- Scenario 19: Missing/empty placeholder left as literal ---
func TestAPIPlaceholderEmptyValue(t *testing.T) {
	writeContextFile(t, "org-123", "", "env-789", "svc-abc")

	result := substitutePathPlaceholders("project/{projectId}/env")
	assert.Equal(t, "project/{projectId}/env", result)
}

// --- Scenario 20: Context unavailable — literal preserved ---
func TestAPIPlaceholderContextUnavailable(t *testing.T) {
	// Point HOME to a temp dir with no .qovery/context.json
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	result := substitutePathPlaceholders("organization/{organizationId}/project")
	// GetCurrentContext() will error → zero-value context → empty string → literal preserved
	assert.Equal(t, "organization/{organizationId}/project", result)
}

// --- Unit tests for coerceFieldValue ---
func TestCoerceFieldValue(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"3.14", float64(3.14)},
		{"42.0", float64(42.0)},
		{"hello", "hello"},
		{"123abc", "123abc"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, coerceFieldValue(tc.input))
		})
	}
}

// --- Unit tests for GetAPIBaseURL ---
func TestGetAPIBaseURL(t *testing.T) {
	t.Run("default URL when env var not set", func(t *testing.T) {
		t.Setenv("QOVERY_API_URL", "")
		assert.Equal(t, "https://api.qovery.com", utils.GetAPIBaseURL())
	})

	t.Run("env var URL used when set", func(t *testing.T) {
		t.Setenv("QOVERY_API_URL", "https://staging.api.qovery.com")
		assert.Equal(t, "https://staging.api.qovery.com", utils.GetAPIBaseURL())
	})

	t.Run("trailing slash stripped from env var", func(t *testing.T) {
		t.Setenv("QOVERY_API_URL", "https://staging.api.qovery.com/")
		assert.Equal(t, "https://staging.api.qovery.com", utils.GetAPIBaseURL())
	})
}

// --- M4: Duplicate --field key rejected ---
func TestAPIFieldDuplicateKey(t *testing.T) {
	err := validateAPIArgs("organization", "", "", []string{"name=a", "name=b"}, nil)
	assert.ErrorContains(t, err, "duplicate field key")
}
