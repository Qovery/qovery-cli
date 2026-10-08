package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"regexp"
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

type pageSnapshot struct {
	Text     string `json:"text"`
	Children []struct {
		Tag  string `json:"tag"`
		Href string `json:"href"`
		Text string `json:"text"`
	} `json:"children"`
	Requests []struct {
		Method string `json:"method"`
		URL    string `json:"url"`
		Async  bool   `json:"async"`
		Sent   bool   `json:"sent"`
	} `json:"requests"`
	Timers      []int    `json:"timers"`
	Navigations []string `json:"navigations"`
}

type pageRun struct {
	Pending       pageSnapshot `json:"pending"`
	AfterResponse pageSnapshot `json:"afterResponse"`
	AfterTimers   pageSnapshot `json:"afterTimers"`
	Error         string       `json:"error"`
}

// runAuthorizationPage serves the real /authorization page, extracts its inline
// script and runs it in node against a fake browser (testdata/auth_page_harness.js).
func runAuthorizationPage(t *testing.T, scenario string) pageRun {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required to run the callback page script")
	}
	stubTokenEndpoint(t)
	ts := httptest.NewServer(newAuthorizationServer("v", func(TokensResponse) {}).Handler)
	defer ts.Close()

	_, page := getStatus(t, ts.URL+"/authorization?code=abc%20123")
	match := regexp.MustCompile(`(?s)<script[^>]*>(.*)</script>`).FindStringSubmatch(page)
	if match == nil {
		t.Fatalf("no inline script in page: %q", page)
	}

	cmd := exec.Command(node, "testdata/auth_page_harness.js", scenario)
	cmd.Stdin = strings.NewReader(match[1])
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("harness failed: %v\n%s", err, stdout)
	}
	var run pageRun
	if err := json.Unmarshal(stdout, &run); err != nil {
		t.Fatalf("harness output %q: %v", stdout, err)
	}
	if run.Error != "" {
		t.Fatalf("page script threw: %s", run.Error)
	}
	return run
}

func assertPendingPage(t *testing.T, run pageRun) {
	t.Helper()
	p := run.Pending
	if p.Text != "Authenticating..." || len(p.Children) != 0 {
		t.Errorf("pending text = %q with %d children, want the Authenticating... placeholder", p.Text, len(p.Children))
	}
	wantURL := fmt.Sprintf("http://localhost:%d/authorization/valid?code=abc%%20123", httpAuthPort)
	if len(p.Requests) != 1 || p.Requests[0].Method != "GET" || p.Requests[0].URL != wantURL || !p.Requests[0].Async || !p.Requests[0].Sent {
		t.Errorf("requests = %+v, want one sent async GET %s", p.Requests, wantURL)
	}
	if len(p.Timers) != 0 || len(p.Navigations) != 0 {
		t.Errorf("pending page scheduled timers %v / navigations %v", p.Timers, p.Navigations)
	}
}

func TestAuthorizationPagePending(t *testing.T) {
	run := runAuthorizationPage(t, "pending")
	assertPendingPage(t, run)
	if got := run.AfterResponse; got.Text != "Authenticating..." || len(got.Timers) != 0 {
		t.Errorf("page changed without an answer: %+v", got)
	}
}

func TestAuthorizationPageSuccessShowsLinkAndSchedulesRedirect(t *testing.T) {
	run := runAuthorizationPage(t, "success")
	assertPendingPage(t, run)

	got := run.AfterResponse
	if !strings.HasPrefix(got.Text, "Authentication successful") {
		t.Errorf("text = %q, want the server's success message", got.Text)
	}
	if len(got.Children) != 1 || got.Children[0].Tag != "a" || got.Children[0].Href != qoveryConsoleUrl || got.Children[0].Text != qoveryConsoleUrl {
		t.Errorf("children = %+v, want one link to %s", got.Children, qoveryConsoleUrl)
	}
	if len(got.Timers) != 1 || got.Timers[0] != 2000 || len(got.Navigations) != 0 {
		t.Errorf("timers = %v, navigations = %v, want one 2000ms timer and no navigation yet", got.Timers, got.Navigations)
	}
	if after := run.AfterTimers; len(after.Navigations) != 1 || after.Navigations[0] != qoveryConsoleUrl {
		t.Errorf("navigations after the timer = %v, want [%s]", after.Navigations, qoveryConsoleUrl)
	}
}

func TestAuthorizationPageHTTPErrorShowsServerMessage(t *testing.T) {
	run := runAuthorizationPage(t, "http-error")
	assertPendingPage(t, run)

	got := run.AfterResponse
	if !strings.Contains(got.Text, "Authentication failed") || strings.Contains(got.Text, "successful") {
		t.Errorf("text = %q, want the failure message", got.Text)
	}
	if len(got.Children) != 0 || len(got.Timers) != 0 || len(run.AfterTimers.Navigations) != 0 {
		t.Errorf("failure must not add a link or redirect: %+v", run.AfterTimers)
	}
}

func TestAuthorizationPageNetworkErrorShowsUnreachableMessage(t *testing.T) {
	run := runAuthorizationPage(t, "network-error")
	assertPendingPage(t, run)

	got := run.AfterResponse
	if !strings.Contains(got.Text, "could not be reached") || strings.Contains(got.Text, "successful") {
		t.Errorf("text = %q, want the unreachable message", got.Text)
	}
	if len(got.Children) != 0 || len(got.Timers) != 0 || len(run.AfterTimers.Navigations) != 0 {
		t.Errorf("failure must not add a link or redirect: %+v", run.AfterTimers)
	}
}
