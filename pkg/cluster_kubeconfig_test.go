package pkg

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetKubeconfigByClusterIdWithError(t *testing.T) {
	tests := []struct {
		name              string
		readOnly          bool
		wantReadOnlyQuery string
		statusCode        int
		wantKubeconfig    string
		wantError         bool
	}{
		{
			name:              "read-only kubeconfig",
			readOnly:          true,
			wantReadOnlyQuery: "true",
			statusCode:        http.StatusOK,
			wantKubeconfig:    "apiVersion: v1\n",
		},
		{
			name:           "read-write kubeconfig",
			statusCode:     http.StatusOK,
			wantKubeconfig: "apiVersion: v1\n",
		},
		{
			name:       "API failure",
			statusCode: http.StatusServiceUnavailable,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.Path, "/organization/00000000-0000-0000-0000-000000000000/cluster/cluster-id/kubeconfig"; got != want {
					t.Errorf("expected request path %q, got %q", want, got)
				}
				if got := r.URL.Query().Get("with_token_from_cli"); got != "true" {
					t.Errorf("expected with_token_from_cli=true, got %q", got)
				}
				if got := r.URL.Query().Get("read_only"); got != tt.wantReadOnlyQuery {
					t.Errorf("expected read_only=%q, got %q", tt.wantReadOnlyQuery, got)
				}
				w.Header().Set("Content-Type", "application/x-yaml")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.wantKubeconfig))
			}))
			defer server.Close()

			t.Setenv("QOVERY_API_URL", server.URL)
			t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "test-token")

			got, err := GetKubeconfigByClusterIdWithError("cluster-id", tt.readOnly)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected an API error")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.wantKubeconfig {
				t.Fatalf("expected kubeconfig %q, got %q", tt.wantKubeconfig, got)
			}
		})
	}
}

func TestGetKubeconfigByClusterIdLegacyUsesReadOnlyRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/organization/00000000-0000-0000-0000-000000000000/cluster/cluster-id/kubeconfig"; got != want {
			t.Errorf("expected request path %q, got %q", want, got)
		}
		if got := r.URL.Query().Get("read_only"); got != "true" {
			t.Errorf("expected read_only=true, got %q", got)
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("apiVersion: v1\n"))
	}))
	defer server.Close()

	t.Setenv("QOVERY_API_URL", server.URL)
	t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "test-token")

	if got, want := GetKubeconfigByClusterId("cluster-id", true), "apiVersion: v1\n"; got != want {
		t.Fatalf("expected kubeconfig %q, got %q", want, got)
	}
}
