package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qovery/qovery-client-go"
)

func TestCheckOrgaValid(t *testing.T) {
	tests := []struct {
		name    string
		results []qovery.Organization
		wantErr bool
	}{
		{"empty organization list returns an error", []qovery.Organization{}, true},
		{"non-empty organization list returns nil", []qovery.Organization{{Id: "org-1", Name: "test"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := qovery.OrganizationResponseList{Results: tt.results}
			err := checkOrgaValid(&list)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkOrgaValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// writeTestQoveryContext writes a minimal, valid ~/.qovery/context.json under home
// so GetCurrentContext() finds a non-expired access token without touching the
// real user's context.
func writeTestQoveryContext(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".qovery")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := QoveryContext{
		AccessToken:           "test-access-token",
		AccessTokenExpiration: time.Now().Add(time.Hour),
		RefreshToken:          "test-refresh-token",
	}
	bytes, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	contextPath := filepath.Join(dir, ContextFileName+".json")
	if err := os.WriteFile(contextPath, bytes, ContextFilePermissions); err != nil {
		t.Fatal(err)
	}
}

// TestGetAccessToken_SkipOrgaCheck locks in the one behavior the qovery-cli#702
// review asked to have covered: GetAccessToken(false) must keep rejecting a
// zero-organization account (the guard every other command relies on), while
// GetAccessToken(true) must let that one case through — used exclusively by
// `qovery api organization --method POST` to bootstrap a brand-new account's
// first organization.
func TestGetAccessToken_SkipOrgaCheck(t *testing.T) {
	tests := []struct {
		name          string
		orgListJSON   string
		skipOrgaCheck bool
		wantErr       bool
	}{
		{"zero organizations, check enforced -> rejected", `{"results":[]}`, false, true},
		{"zero organizations, check skipped -> allowed", `{"results":[]}`, true, false},
		{"existing organization, check enforced -> allowed", `{"results":[{"id":"org-1","created_at":"2024-01-01T00:00:00Z","name":"test","plan":"BUSINESS_2025"}]}`, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.orgListJSON))
			}))
			defer server.Close()

			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("QOVERY_API_URL", server.URL)
			t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "")
			t.Setenv("Q_CLI_ACCESS_TOKEN", "")
			writeTestQoveryContext(t, home)

			_, _, err := GetAccessToken(tt.skipOrgaCheck)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetAccessToken(%v) error = %v, wantErr %v", tt.skipOrgaCheck, err, tt.wantErr)
			}
		})
	}
}
