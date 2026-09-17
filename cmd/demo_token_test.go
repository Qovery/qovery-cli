package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qovery/qovery-cli/utils"
)

func TestWriteDemoTokenFileWithRelativeDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("credentials", 0700); err != nil {
		t.Fatal(err)
	}
	path, err := writeDemoTokenFile("credentials", "Bearer", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("token path must be absolute, got %q", path)
	}
	// Both demo scripts change directory before making API requests.
	t.Chdir(t.TempDir())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Authorization: Bearer test-token\n" {
		t.Fatalf("unexpected authorization header: %q", data)
	}
}

func TestDemoScriptsReadTokenFile(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is required to test the demo scripts")
	}

	for _, tokenType := range []utils.AccessTokenType{"Bearer", "Token"} {
		for _, command := range []string{"up", "destroy"} {
			t.Run(string(tokenType)+"/"+command, func(t *testing.T) {
				token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.test-signature"
				if tokenType == "Token" {
					token = "qov_test-static-token-secret"
				}
				// Include spaces to exercise quoting of the header file path.
				dir := filepath.Join(t.TempDir(), "demo credentials")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				tokenPath, err := writeDemoTokenFile(dir, tokenType, utils.AccessToken(token))
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(tokenPath)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0600 {
					t.Fatalf("token file permissions = %o, want 600", info.Mode().Perm())
				}

				headers := make(chan string, 8)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					headers <- r.Header.Get("Authorization")
					_, _ = w.Write([]byte("{}"))
				}))
				defer server.Close()

				// Run the scripts' real API functions with curl against the local
				// server, without installing dependencies or modifying a cluster.
				script := demoScriptsCreate
				shell := "bash"
				args := []string{"local-demo", "AMD64", "org-id", tokenPath, "true", "CLI test"}
				calls := `
jq() {
  cat >/dev/null
  case "$*" in
    *'.results[0].id') printf 'null\n' ;;
    *'select('*) ;;
    *'.id') printf 'test-id\n' ;;
  esac
}
get_or_create_on_premise_account
get_or_create_demo_cluster test-account "$CLUSTER_NAME"
get_cluster_values test-cluster
`
				wantRequests := 5
				if command == "destroy" {
					script = demoScriptsDestroy
					shell = "sh"
					args = []string{"local-demo", "org-id", tokenPath, "true"}
					calls = `
jq() { cat >/dev/null; printf 'test-cluster\n'; }
delete_qovery_demo_cluster "$CLUSTER_NAME"
`
					wantRequests = 2
				}
				if _, err := exec.LookPath(shell); err != nil {
					t.Skipf("%s is required to test the demo script", shell)
				}
				definitions, _, found := strings.Cut(string(script), "# shellcheck disable=SC2046")
				if !found {
					t.Fatal("could not separate script definitions from cluster setup")
				}
				scriptPath := filepath.Join(dir, "test-demo.sh")
				if err := os.WriteFile(scriptPath, []byte(definitions+calls), 0700); err != nil {
					t.Fatal(err)
				}
				shCmd := exec.Command(shell, append([]string{"-x", scriptPath}, args...)...)
				shCmd.Env = append(os.Environ(), "QOVERY_API_URL="+server.URL)
				if strings.Contains(shCmd.String(), token) {
					t.Fatal("token leaked into the command line")
				}
				output, err := shCmd.CombinedOutput()
				if bytes.Contains(output, []byte(token)) {
					t.Fatal("token leaked into script output or debug traces")
				}
				if err != nil {
					t.Fatalf("demo API calls failed: %v\n%s", err, output)
				}
				if len(headers) != wantRequests {
					t.Fatalf("got %d API requests, want %d\n%s", len(headers), wantRequests, output)
				}
				for range wantRequests {
					if got := <-headers; got != string(tokenType)+" "+token {
						t.Fatalf("API authorization = %q, want the token from the file", got)
					}
				}
			})
		}
	}
}
