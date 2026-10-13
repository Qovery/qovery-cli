package pkg

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qovery/qovery-cli/utils"
)

const organizationListJSON = `{"results":[{"id":"org-1","created_at":"2024-01-01T00:00:00Z","name":"test","plan":"BUSINESS_2025"}]}`

// authFixture counts the interactive logins triggered by the retry helper, whose browser flow is stubbed out
type authFixture struct {
	interactiveLogins int
}

// setupAuth isolates the test from the real user's shell and context file, serves the organization probe made by
// utils.GetAccessToken from a local server so nothing reaches the Qovery API, and stubs the interactive login
func setupAuth(t *testing.T, organizationHandler http.HandlerFunc) *authFixture {
	t.Helper()
	server := httptest.NewServer(organizationHandler)
	t.Cleanup(server.Close)

	// os.UserHomeDir reads HOME on unix and USERPROFILE on windows: point both at a scratch directory
	// so the stored context never touches the developer's real ~/.qovery/context.json
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("QOVERY_API_URL", server.URL)
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "")
	t.Setenv("Q_CLI_ACCESS_TOKEN", "")

	fixture := &authFixture{}
	previous := requestUserToAuthenticate
	requestUserToAuthenticate = func() { fixture.interactiveLogins++ }
	t.Cleanup(func() { requestUserToAuthenticate = previous })
	return fixture
}

// writeStoredContext writes ~/.qovery/context.json under the test HOME with a stored access token.
// An empty refresh token makes utils.RefreshAccessToken fail before any call to the authentication server.
func writeStoredContext(t *testing.T, refreshToken string) {
	t.Helper()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	dir := filepath.Join(home, ".qovery")
	require.NoError(t, os.MkdirAll(dir, 0700))

	storedContext := utils.QoveryContext{
		AccessToken:           "stored-access-token",
		AccessTokenExpiration: time.Now().Add(time.Hour),
		RefreshToken:          utils.RefreshToken(refreshToken),
	}
	bytes, err := json.Marshal(storedContext)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, utils.ContextFileName+".json"), bytes, utils.ContextFilePermissions))
}

func serveJSON(status int, body string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}
}

func noAPICallExpected(t *testing.T) http.HandlerFunc {
	return func(_ http.ResponseWriter, request *http.Request) {
		t.Errorf("unexpected call to the Qovery API: %s", request.URL.Path)
	}
}

func authorized() (*string, *http.Response, error) {
	result := "ok"
	return &result, &http.Response{StatusCode: http.StatusOK}, nil
}

// unauthorized mirrors the generated client, which returns an error alongside any non-2xx response
func unauthorized() (*string, *http.Response, error) {
	return nil, &http.Response{StatusCode: http.StatusUnauthorized}, errors.New("401 Unauthorized")
}

func TestRetryQoveryClientApiRequestOnUnauthorized(t *testing.T) {
	t.Run("Should not retry when first request is authorized", func(t *testing.T) {
		// given
		fixture := setupAuth(t, noAPICallExpected(t))
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			return authorized()
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false}, calls)
		assert.Equal(t, 0, fixture.interactiveLogins)
	})

	t.Run("Should fail without refresh nor interactive auth when the rejected token comes from the environment", func(t *testing.T) {
		for _, envVar := range []string{"QOVERY_CLI_ACCESS_TOKEN", "Q_CLI_ACCESS_TOKEN"} {
			t.Run(envVar, func(t *testing.T) {
				// given
				fixture := setupAuth(t, noAPICallExpected(t))
				t.Setenv(envVar, "env-token")
				var calls []bool
				request := func(needToRefetchClient bool) (*string, *http.Response, error) {
					calls = append(calls, needToRefetchClient)
					return unauthorized()
				}

				// when
				result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

				// then
				assert.ErrorContains(t, err, "QOVERY_CLI_ACCESS_TOKEN")
				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Nil(t, result)
				assert.Equal(t, []bool{false}, calls)
				assert.Equal(t, 0, fixture.interactiveLogins)
			})
		}
	})

	t.Run("Should refetch client silently without interactive auth when the stored token is accepted", func(t *testing.T) {
		// given
		fixture := setupAuth(t, serveJSON(http.StatusOK, organizationListJSON))
		writeStoredContext(t, "stored-refresh-token")
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if !needToRefetchClient {
				return unauthorized()
			}
			return authorized()
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false, true}, calls)
		assert.Equal(t, 0, fixture.interactiveLogins)
	})

	t.Run("Should return transport error without interactive auth when token refresh succeeded", func(t *testing.T) {
		// given
		fixture := setupAuth(t, serveJSON(http.StatusOK, organizationListJSON))
		writeStoredContext(t, "stored-refresh-token")
		var calls []bool
		transportErr := errors.New("dial tcp: i/o timeout")
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if !needToRefetchClient {
				return unauthorized()
			}
			return nil, nil, transportErr
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.ErrorIs(t, err, transportErr)
		assert.Nil(t, response)
		assert.Nil(t, result)
		assert.Equal(t, []bool{false, true}, calls)
		assert.Equal(t, 0, fixture.interactiveLogins)
	})

	t.Run("Should surface the token error without interactive auth when signing in again cannot fix it", func(t *testing.T) {
		// given: the stored token is accepted but the account has no organization
		fixture := setupAuth(t, serveJSON(http.StatusOK, `{"results":[]}`))
		writeStoredContext(t, "stored-refresh-token")
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			return unauthorized()
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.ErrorContains(t, err, "you don't have any organization")
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Nil(t, result)
		assert.Equal(t, []bool{false}, calls)
		assert.Equal(t, 0, fixture.interactiveLogins)
	})

	t.Run("Should ask for interactive auth when the stored credentials cannot be refreshed", func(t *testing.T) {
		// given: the stored token is rejected and there is no refresh token to get a new one
		fixture := setupAuth(t, serveJSON(http.StatusUnauthorized, `{}`))
		writeStoredContext(t, "")
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if !needToRefetchClient {
				return unauthorized()
			}
			return authorized()
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false, true}, calls)
		assert.Equal(t, 1, fixture.interactiveLogins)
	})

	t.Run("Should ask for interactive auth when the request is still unauthorized after a silent refresh", func(t *testing.T) {
		// given
		fixture := setupAuth(t, serveJSON(http.StatusOK, organizationListJSON))
		writeStoredContext(t, "stored-refresh-token")
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if len(calls) <= 2 {
				return unauthorized()
			}
			return authorized()
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false, true, true}, calls)
		assert.Equal(t, 1, fixture.interactiveLogins)
	})
}
