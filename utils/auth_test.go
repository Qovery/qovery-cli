package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testBrowserClient  = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	testHeadlessClient = "f9drkTNpxsEw2VU2PVDrxhyT3vVuFT0Y"
	testOrgListJSON    = `{"results":[{"id":"org-1","created_at":"2024-01-01T00:00:00Z","name":"test","plan":"BUSINESS_2025"}]}`
)

// accessTokenIssuedBy builds an access token shaped like the ones Auth0 issues: a JWT whose azp claim is the client
// the user signed in with. An empty client leaves the azp claim out.
func accessTokenIssuedBy(t *testing.T, client string) AccessToken {
	t.Helper()
	claims := jwt.MapClaims{"sub": "user-1"}
	if client != "" {
		claims["azp"] = client
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	return AccessToken(token)
}

// setupExpiredSession stores accessToken under a scratch HOME and serves both the Qovery API and the authentication
// server locally. The API rejects the stored token, as it does once the token has expired, and accepts "refreshed-token".
func setupExpiredSession(t *testing.T, accessToken AccessToken, tokenEndpoint http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/oauth/token":
			tokenEndpoint(w, r)
		case r.Header.Get("Authorization") == "Bearer refreshed-token":
			_, _ = w.Write([]byte(testOrgListJSON))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)

	previousEndpoint := oAuthTokenEndpoint
	oAuthTokenEndpoint = server.URL + "/oauth/token"
	t.Cleanup(func() { oAuthTokenEndpoint = previousEndpoint })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("QOVERY_API_URL", server.URL)
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "")
	t.Setenv("Q_CLI_ACCESS_TOKEN", "")

	dir := filepath.Join(home, ".qovery")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	bytes, err := json.Marshal(QoveryContext{
		AccessToken:           accessToken,
		AccessTokenExpiration: time.Now().Add(-time.Minute),
		RefreshToken:          "test-refresh-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ContextFileName+".json"), bytes, ContextFilePermissions); err != nil {
		t.Fatal(err)
	}
}

// A refresh token only works with the client it was issued to: 'qovery auth --headless' signs in with another client
// than 'qovery auth', so the refresh has to follow the client of the stored session
func TestGetAccessToken_RefreshesWithTheClientThatIssuedTheStoredToken(t *testing.T) {
	tests := []struct {
		name        string
		storedToken func(t *testing.T) AccessToken
		wantClient  string
	}{
		{"session opened with 'qovery auth --headless'", func(t *testing.T) AccessToken { return accessTokenIssuedBy(t, testHeadlessClient) }, testHeadlessClient},
		{"session opened with 'qovery auth'", func(t *testing.T) AccessToken { return accessTokenIssuedBy(t, testBrowserClient) }, testBrowserClient},
		{"stored token without the azp claim", func(t *testing.T) AccessToken { return accessTokenIssuedBy(t, "") }, testBrowserClient},
		{"stored token that is not a JWT", func(t *testing.T) AccessToken { return "opaque-access-token" }, testBrowserClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			var gotClient, gotGrantType, gotRefreshToken string
			setupExpiredSession(t, tt.storedToken(t), func(w http.ResponseWriter, r *http.Request) {
				gotClient = r.PostFormValue("client_id")
				gotGrantType = r.PostFormValue("grant_type")
				gotRefreshToken = r.PostFormValue("refresh_token")
				_, _ = w.Write([]byte(`{"access_token":"refreshed-token","expires_in":900}`))
			})

			// when
			tokenType, token, err := GetAccessToken(false)

			// then
			if err != nil {
				t.Fatalf("GetAccessToken() unexpected error: %v", err)
			}
			if tokenType != "Bearer" || token != "refreshed-token" {
				t.Fatalf("GetAccessToken() = %q %q, want the refreshed bearer token", tokenType, token)
			}
			if gotClient != tt.wantClient {
				t.Fatalf("refresh sent client_id %q, want %q", gotClient, tt.wantClient)
			}
			if gotGrantType != "refresh_token" || gotRefreshToken != "test-refresh-token" {
				t.Fatalf("refresh sent grant_type %q and refresh_token %q", gotGrantType, gotRefreshToken)
			}
			stored, err := GetCurrentContext()
			if err != nil {
				t.Fatal(err)
			}
			if stored.AccessToken != "refreshed-token" || stored.RefreshToken != "test-refresh-token" {
				t.Fatalf("stored context = access token %q, refresh token %q", stored.AccessToken, stored.RefreshToken)
			}
		})
	}
}

func TestGetAccessToken_FailedRefreshKeepsTheStoredSession(t *testing.T) {
	tests := []struct {
		name               string
		status             int
		body               string
		wantSignInRequired bool
		wantErrContains    string
	}{
		{
			"refresh token rejected -> sign-in required",
			http.StatusForbidden,
			`{"error":"invalid_grant","error_description":"Unknown or invalid refresh token."}`,
			true,
			"Unknown or invalid refresh token.",
		},
		{
			"refresh request refused with 400 -> sign-in required",
			http.StatusBadRequest,
			`{"error":"invalid_request","error_description":"Missing required parameter: refresh_token"}`,
			true,
			"Missing required parameter: refresh_token",
		},
		{
			"refresh request refused with 401 -> sign-in required",
			http.StatusUnauthorized,
			`{"error":"access_denied","error_description":"Unauthorized"}`,
			true,
			"Unauthorized",
		},
		{
			"rejection without a description -> sign-in required, error code reported",
			http.StatusForbidden,
			`{"error":"invalid_grant"}`,
			true,
			"invalid_grant",
		},
		{
			"rejection without a readable body -> sign-in required, status reported",
			http.StatusForbidden,
			`not json`,
			true,
			"403 Forbidden",
		},
		{
			"authentication server unavailable -> transient error, not sign-in",
			http.StatusServiceUnavailable,
			`{}`,
			false,
			"503",
		},
		{
			"answer without an access token -> transient error, not sign-in",
			http.StatusOK,
			`{}`,
			false,
			"no access token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			storedToken := accessTokenIssuedBy(t, testHeadlessClient)
			setupExpiredSession(t, storedToken, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			// when
			_, token, err := GetAccessToken(false)

			// then
			if token != "" {
				t.Fatalf("GetAccessToken() token = %q, want none", token)
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("GetAccessToken() error = %v, want it to contain %q", err, tt.wantErrContains)
			}
			if got := errors.Is(err, ErrSignInRequired); got != tt.wantSignInRequired {
				t.Fatalf("errors.Is(err, ErrSignInRequired) = %v, want %v (err: %v)", got, tt.wantSignInRequired, err)
			}
			stored, err := GetCurrentContext()
			if err != nil {
				t.Fatal(err)
			}
			if stored.AccessToken != storedToken {
				t.Fatalf("a failed refresh replaced the stored access token with %q", stored.AccessToken)
			}
		})
	}
}
