package pkg

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// exchangeRecorder records the code_verifier of every token exchange. The
// handler runs on server goroutines, so reads go through snapshot.
type exchangeRecorder struct {
	mu        sync.Mutex
	exchanges []string
}

func (r *exchangeRecorder) record(entry string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges = append(r.exchanges, entry)
}

func (r *exchangeRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.exchanges...)
}

// stubTokenEndpoint answers every token exchange with a success.
func stubTokenEndpoint(t *testing.T) *exchangeRecorder {
	t.Helper()
	return stubTokenEndpointWith(t, http.StatusOK, `{"access_token":"at","refresh_token":"rt","expires_in":3600}`)
}

func stubTokenEndpointWith(t *testing.T, status int, body string) *exchangeRecorder {
	t.Helper()
	recorder := &exchangeRecorder{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		recorder.record(r.PostForm.Get("code_verifier") + "|" + r.PostForm.Get("code"))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)

	prevEndpoint, prevDelay := oAuthTokenEndpoint, authShutdownDelay
	oAuthTokenEndpoint = ts.URL
	authShutdownDelay = 0
	t.Cleanup(func() { oAuthTokenEndpoint, authShutdownDelay = prevEndpoint, prevDelay })
	return recorder
}

func getStatus(t *testing.T, url string) (int, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func get(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return string(body)
}

func TestAuthorizationServerRepeatedAttemptsUseTheirOwnVerifier(t *testing.T) {
	verifiers := stubTokenEndpoint(t)

	// Reuse the same address for every attempt, like the fixed callback port.
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := first.Addr().String()
	listener := first

	var stored []TokensResponse
	for i, verifier := range []string{"verifier-1", "verifier-2", "verifier-3"} {
		srv := newAuthorizationServer(verifier, func(tokens TokensResponse) { stored = append(stored, tokens) })

		done := make(chan error, 1)
		// A second attempt used to panic here on the duplicate "/authorization" pattern.
		go func() { done <- srv.Serve(listener) }()

		code := "code-" + verifier
		if body := get(t, "http://"+addr+"/authorization?code="+code); body == "" {
			t.Fatalf("attempt %d: empty /authorization body", i)
		}
		get(t, "http://"+addr+"/authorization/valid?code="+code)

		select {
		case err := <-done:
			if err != http.ErrServerClosed {
				t.Fatalf("attempt %d: Serve returned %v, want ErrServerClosed", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("attempt %d: server did not shut down after the callback", i)
		}

		// The port must be free for the next attempt.
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("attempt %d: port not released after shutdown: %v", i, err)
		}
	}
	_ = listener.Close()

	want := []string{"verifier-1|code-verifier-1", "verifier-2|code-verifier-2", "verifier-3|code-verifier-3"}
	got := verifiers.snapshot()
	if len(got) != len(want) {
		t.Fatalf("token exchanges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("exchange %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(stored) != 3 || stored[0].AccessToken != "at" || stored[0].RefreshToken != "rt" {
		t.Errorf("stored tokens = %+v", stored)
	}
}

func TestAuthorizationServersDoNotShareHandlers(t *testing.T) {
	verifiers := stubTokenEndpoint(t)

	a := httptest.NewServer(newAuthorizationServer("verifier-a", func(TokensResponse) {}).Handler)
	defer a.Close()
	b := httptest.NewServer(newAuthorizationServer("verifier-b", func(TokensResponse) {}).Handler)
	defer b.Close()

	get(t, b.URL+"/authorization/valid?code=c")
	get(t, a.URL+"/authorization/valid?code=c")

	want := []string{"verifier-b|c", "verifier-a|c"}
	got := verifiers.snapshot()
	if len(got) != len(want) {
		t.Fatalf("token exchanges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("exchange %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAuthorizationServerDoesNotUseDefaultServeMux(t *testing.T) {
	stubTokenEndpoint(t)
	_ = newAuthorizationServer("v", func(TokensResponse) {})
	_ = newAuthorizationServer("v", func(TokensResponse) {})

	if _, pattern := http.DefaultServeMux.Handler(httptest.NewRequest(http.MethodGet, "/authorization", nil)); pattern != "" {
		t.Errorf("/authorization registered on http.DefaultServeMux (pattern %q)", pattern)
	}
}

func TestAuthorizationServerRejectedExchangeStoresNothing(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"non-200 answer":     {http.StatusForbidden, `{"error":"invalid_grant"}`},
		"200, empty tokens":  {http.StatusOK, `{}`},
		"200, invalid JSON":  {http.StatusOK, `not json`},
		"200, error payload": {http.StatusOK, `{"error":"invalid_grant"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stubTokenEndpointWith(t, tc.status, tc.body)

			var exits []int
			prevExit := exitProcess
			exitProcess = func(code int) { exits = append(exits, code) }
			t.Cleanup(func() { exitProcess = prevExit })

			stored := 0
			ts := httptest.NewServer(newAuthorizationServer("v", func(TokensResponse) { stored++ }).Handler)
			defer ts.Close()

			res, err := http.Get(ts.URL + "/authorization/valid?code=stale")
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()

			if stored != 0 {
				t.Errorf("tokens stored %d time(s) after a rejected exchange", stored)
			}
			if len(exits) != 1 || exits[0] != 1 {
				t.Errorf("exit calls = %v, want exactly one exit with status 1", exits)
			}
			if res.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", res.StatusCode, http.StatusUnauthorized)
			}
			if body := string(body); strings.Contains(body, "successful") || !strings.Contains(body, "failed") {
				t.Errorf("browser body = %q, want a failure message", body)
			}
		})
	}
}

func TestAuthorizationServerMissingCodeIsABadRequest(t *testing.T) {
	exchanges := stubTokenEndpoint(t)
	ts := httptest.NewServer(newAuthorizationServer("v", func(TokensResponse) { t.Error("tokens stored") }).Handler)
	defer ts.Close()

	for _, path := range []string{"/authorization/valid", "/authorization/valid?code="} {
		status, body := getStatus(t, ts.URL+path)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", path, status, http.StatusBadRequest)
		}
		if strings.Contains(body, "successful") {
			t.Errorf("%s: body = %q, want a failure message", path, body)
		}
	}
	if got := exchanges.snapshot(); len(got) != 0 {
		t.Errorf("token exchanges = %v, want none", got)
	}
}

// The callback page is served before the token exchange runs, so it must not
// claim success itself.
func TestAuthorizationPageDoesNotClaimSuccess(t *testing.T) {
	exchanges := stubTokenEndpoint(t)
	ts := httptest.NewServer(newAuthorizationServer("v", func(TokensResponse) {}).Handler)
	defer ts.Close()

	status, body := getStatus(t, ts.URL+"/authorization?code=abc")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if strings.Contains(strings.ToLower(body), "successful") {
		t.Errorf("page claims success before the exchange: %q", body)
	}
	if !strings.Contains(body, "/authorization/valid") {
		t.Errorf("page does not call /authorization/valid: %q", body)
	}
	if got := exchanges.snapshot(); len(got) != 0 {
		t.Errorf("serving the page triggered token exchanges: %v", got)
	}
}

func TestAuthorizationServerSuccessfulExchangeAnswersSuccess(t *testing.T) {
	stubTokenEndpoint(t)
	var stored []TokensResponse
	ts := httptest.NewServer(newAuthorizationServer("v", func(tokens TokensResponse) { stored = append(stored, tokens) }).Handler)
	defer ts.Close()

	status, body := getStatus(t, ts.URL+"/authorization/valid?code=ok")
	if status != http.StatusOK || !strings.Contains(body, "Authentication successful") {
		t.Errorf("status = %d, body = %q, want 200 with the success message", status, body)
	}
	if len(stored) != 1 {
		t.Errorf("stored tokens = %+v, want one entry", stored)
	}
}
