package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
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

// tokenStore collects stored tokens. onTokens runs on a server goroutine, so
// the test reads them through snapshot.
type tokenStore struct {
	mu     sync.Mutex
	tokens []TokensResponse
}

func (s *tokenStore) store(tokens TokensResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = append(s.tokens, tokens)
}

func (s *tokenStore) snapshot() []TokensResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]TokensResponse(nil), s.tokens...)
}

// trackingListener remembers accepted connections so a test can sever them the
// way a dying process would.
type trackingListener struct {
	net.Listener
	// gate, when set, holds back every write on accepted connections until it
	// is closed. It simulates a slow client.
	gate  chan struct{}
	mu    sync.Mutex
	conns []net.Conn
}

// gatedConn blocks writes until the gate is closed.
type gatedConn struct {
	net.Conn
	gate <-chan struct{}
}

func (c *gatedConn) Write(b []byte) (int, error) {
	<-c.gate
	return c.Conn.Write(b)
}

func (l *trackingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil && l.gate != nil {
		conn = &gatedConn{Conn: conn, gate: l.gate}
	}
	if err == nil {
		l.mu.Lock()
		l.conns = append(l.conns, conn)
		l.mu.Unlock()
	}
	return conn, err
}

func (l *trackingListener) killConnections() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, conn := range l.conns {
		_ = conn.Close()
	}
}

// stubExit replaces exitProcess with a channel for tests that only mount the
// handler. The handler must never exit on these paths.
func stubExit(t *testing.T) chan int {
	t.Helper()
	exits := make(chan int, 4)
	prevExit := exitProcess
	exitProcess = func(code int) { exits <- code }
	t.Cleanup(func() { exitProcess = prevExit })
	return exits
}

// runningAuthorization is a callback server run the way DoRequestUserToAuthenticate
// runs it, through serveAuthorization. The exit stub severs every connection
// before it reports the code, like the real process exit does: a response that
// was not completed before the exit reaches the client truncated.
type runningAuthorization struct {
	url   string
	exits chan int
	done  chan struct{}
}

func startAuthorization(t *testing.T, onTokens func(TokensResponse)) *runningAuthorization {
	t.Helper()
	return startAuthorizationOn(t, onTokens, nil)
}

func startAuthorizationOn(t *testing.T, onTokens func(TokensResponse), gate chan struct{}) *runningAuthorization {
	t.Helper()
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := &trackingListener{Listener: inner, gate: gate}
	running := &runningAuthorization{
		url:   "http://" + inner.Addr().String(),
		exits: make(chan int, 4),
		done:  make(chan struct{}),
	}
	prevExit := exitProcess
	exitProcess = func(code int) {
		listener.killConnections()
		running.exits <- code
	}
	srv := newAuthorizationServer("v", onTokens)
	go func() {
		defer close(running.done)
		serveAuthorization(srv, listener)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		<-running.done
		exitProcess = prevExit
	})
	return running
}

func waitForExit(t *testing.T, exits <-chan int, want int) {
	t.Helper()
	select {
	case code := <-exits:
		if code != want {
			t.Errorf("exit status = %d, want %d", code, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exitProcess was not called")
	}
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

	store := &tokenStore{}
	for i, verifier := range []string{"verifier-1", "verifier-2", "verifier-3"} {
		srv := newAuthorizationServer(verifier, store.store)

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
	if stored := store.snapshot(); len(stored) != 3 || stored[0].AccessToken != "at" || stored[0].RefreshToken != "rt" {
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
		"non-200 answer":                          {http.StatusForbidden, `{"error":"invalid_grant"}`},
		"200, empty tokens":                       {http.StatusOK, `{}`},
		"200, invalid JSON":                       {http.StatusOK, `not json`},
		"200, error payload":                      {http.StatusOK, `{"error":"invalid_grant"}`},
		"200, valid tokens then trailing garbage": {http.StatusOK, `{"access_token":"at","refresh_token":"rt","expires_in":3600} not json`},
		"200, valid tokens then a second value":   {http.StatusOK, `{"access_token":"at","refresh_token":"rt","expires_in":3600}{}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stubTokenEndpointWith(t, tc.status, tc.body)
			var stored atomic.Int32
			a := startAuthorization(t, func(TokensResponse) { stored.Add(1) })

			res, body := getComplete(t, a.url+"/authorization/valid?code=stale")
			assertCompleteFailure(t, res, body, http.StatusUnauthorized)
			waitForExit(t, a.exits, 1)
			<-a.done
			if extra := len(a.exits); extra != 0 {
				t.Errorf("%d extra exit call(s), want exactly one", extra)
			}
			if n := stored.Load(); n != 0 {
				t.Errorf("tokens stored %d time(s) after a rejected exchange", n)
			}
		})
	}
}

// getComplete reads the whole response and fails if the connection was cut
// before the body was complete.
// The graceful shutdown gives up after authShutdownTimeout, but a failure
// response still being written must not be cut by the exit that follows.
func TestAuthorizationServerFailureExitWaitsForSlowDelivery(t *testing.T) {
	stubTokenEndpointWith(t, http.StatusForbidden, `{"error":"invalid_grant"}`)
	prevTimeout := authShutdownTimeout
	authShutdownTimeout = 20 * time.Millisecond
	t.Cleanup(func() { authShutdownTimeout = prevTimeout })

	gate := make(chan struct{})
	var release sync.Once
	releaseGate := func() { release.Do(func() { close(gate) }) }
	t.Cleanup(releaseGate)
	a := startAuthorizationOn(t, func(TokensResponse) {}, gate)

	type result struct {
		res  *http.Response
		body string
		err  error
	}
	delivered := make(chan result, 1)
	go func() {
		res, err := http.Get(a.url + "/authorization/valid?code=stale")
		if err != nil {
			delivered <- result{err: err}
			return
		}
		defer func() { _ = res.Body.Close() }()
		body, err := io.ReadAll(res.Body)
		delivered <- result{res, string(body), err}
	}()

	// Well past the graceful timeout, the response is still held back.
	select {
	case code := <-a.exits:
		t.Fatalf("exitProcess(%d) ran while the failure response was not delivered", code)
	case got := <-delivered:
		t.Fatalf("response arrived while the connection was gated: %+v", got)
	case <-time.After(300 * time.Millisecond):
	}

	releaseGate()
	got := <-delivered
	if got.err != nil {
		t.Fatalf("response truncated: %v (got %q)", got.err, got.body)
	}
	assertCompleteFailure(t, got.res, got.body, http.StatusUnauthorized)
	waitForExit(t, a.exits, 1)
	<-a.done
}

func getComplete(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("GET %s: response truncated: %v (got %q)", url, err, body)
	}
	return res, string(body)
}

func assertCompleteFailure(t *testing.T, res *http.Response, body string, wantStatus int) {
	t.Helper()
	if res.StatusCode != wantStatus {
		t.Errorf("status = %d, want %d", res.StatusCode, wantStatus)
	}
	if want := authFailureMessage + "\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if res.ContentLength != int64(len(body)) {
		t.Errorf("Content-Length = %d, want %d: the response is not framed as complete", res.ContentLength, len(body))
	}
}

func TestAuthorizationServerMissingCodeFailsAndExits(t *testing.T) {
	for _, path := range []string{"/authorization/valid", "/authorization/valid?code="} {
		t.Run(path, func(t *testing.T) {
			exchanges := stubTokenEndpoint(t)
			var stored atomic.Int32
			a := startAuthorization(t, func(TokensResponse) { stored.Add(1) })

			res, body := getComplete(t, a.url+path)
			assertCompleteFailure(t, res, body, http.StatusBadRequest)
			waitForExit(t, a.exits, 1)
			<-a.done
			if extra := len(a.exits); extra != 0 {
				t.Errorf("%d extra exit call(s), want exactly one", extra)
			}
			if got := exchanges.snapshot(); len(got) != 0 {
				t.Errorf("token exchanges = %v, want none", got)
			}
			if n := stored.Load(); n != 0 {
				t.Errorf("tokens stored %d time(s) without a code", n)
			}
		})
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
	store := &tokenStore{}
	ts := httptest.NewServer(newAuthorizationServer("v", store.store).Handler)
	defer ts.Close()

	status, body := getStatus(t, ts.URL+"/authorization/valid?code=ok")
	if status != http.StatusOK || body != authSuccessMessage {
		t.Errorf("status = %d, body = %q, want 200 with the success message", status, body)
	}
	if stored := store.snapshot(); len(stored) != 1 {
		t.Errorf("stored tokens = %+v, want one entry", stored)
	}
}

// A repeated callback must not replay the consumed code: with a real token
// endpoint it would fail and take the CLI down after the tokens were stored.
func TestAuthorizationServerRepeatedCallbackIsOneShot(t *testing.T) {
	exchanges := stubTokenEndpoint(t)
	exits := stubExit(t)
	store := &tokenStore{}
	ts := httptest.NewServer(newAuthorizationServer("v", store.store).Handler)
	defer ts.Close()

	for _, path := range []string{"/authorization/valid?code=ok", "/authorization/valid?code=ok", "/authorization/valid?code=other", "/authorization/valid"} {
		if status, body := getStatus(t, ts.URL+path); status != http.StatusOK || body != authSuccessMessage {
			t.Errorf("%s: status = %d, body = %q, want the first success again", path, status, body)
		}
	}
	if got := exchanges.snapshot(); len(got) != 1 {
		t.Errorf("token exchanges = %v, want exactly one", got)
	}
	if stored := store.snapshot(); len(stored) != 1 {
		t.Errorf("stored tokens = %+v, want one entry", stored)
	}
	ts.Close()
	if len(exits) != 0 {
		t.Errorf("a repeated callback exited the process")
	}
}

func TestAuthorizationServerConcurrentCallbacksExchangeOnce(t *testing.T) {
	exchanges := stubTokenEndpoint(t)
	exits := stubExit(t)
	store := &tokenStore{}
	ts := httptest.NewServer(newAuthorizationServer("v", store.store).Handler)
	defer ts.Close()

	const callers = 8
	type answer struct {
		status int
		body   string
		err    error
	}
	answers := make(chan answer, callers)
	for i := 0; i < callers; i++ {
		go func() {
			res, err := http.Get(ts.URL + "/authorization/valid?code=ok")
			if err != nil {
				answers <- answer{err: err}
				return
			}
			defer func() { _ = res.Body.Close() }()
			body, err := io.ReadAll(res.Body)
			answers <- answer{status: res.StatusCode, body: string(body), err: err}
		}()
	}
	for i := 0; i < callers; i++ {
		got := <-answers
		if got.err != nil || got.status != http.StatusOK || got.body != authSuccessMessage {
			t.Errorf("concurrent callback: status = %d, body = %q, err = %v, want the success answer", got.status, got.body, got.err)
		}
	}
	if got := exchanges.snapshot(); len(got) != 1 {
		t.Errorf("token exchanges = %v, want exactly one", got)
	}
	if stored := store.snapshot(); len(stored) != 1 {
		t.Errorf("stored tokens = %+v, want one entry", stored)
	}
	ts.Close()
	if len(exits) != 0 {
		t.Errorf("a concurrent callback exited the process")
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
	// The page runs against what the real callback server answers, not a copy
	// of its strings.
	var answerStatus int
	var answerBody string
	switch scenario {
	case "success":
		stubTokenEndpoint(t)
		answerStatus, answerBody = callbackAnswer(t)
	case "http-error":
		stubTokenEndpointWith(t, http.StatusForbidden, `{"error":"invalid_grant"}`)
		answerStatus, answerBody = callbackAnswer(t)
	default:
		stubTokenEndpoint(t)
	}
	ts := httptest.NewServer(newAuthorizationServer("v", func(TokensResponse) {}).Handler)
	defer ts.Close()

	_, page := getStatus(t, ts.URL+"/authorization?code=abc%20123")
	match := regexp.MustCompile(`(?s)<script[^>]*>(.*)</script>`).FindStringSubmatch(page)
	if match == nil {
		t.Fatalf("no inline script in page: %q", page)
	}

	cmd := exec.Command(node, "testdata/auth_page_harness.js", scenario)
	cmd.Stdin = strings.NewReader(match[1])
	cmd.Env = append(os.Environ(), fmt.Sprintf("AUTH_RESPONSE_STATUS=%d", answerStatus), "AUTH_RESPONSE_BODY="+answerBody)
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

// callbackAnswer asks a real callback server for its /authorization/valid answer.
func callbackAnswer(t *testing.T) (int, string) {
	t.Helper()
	a := startAuthorization(t, func(TokensResponse) {})
	return getStatus(t, a.url+"/authorization/valid?code=abc")
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
	if got.Text != authSuccessMessage {
		t.Errorf("text = %q, want the server's success message %q", got.Text, authSuccessMessage)
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
	if got.Text != authFailureMessage+"\n" {
		t.Errorf("text = %q, want the server's failure message %q", got.Text, authFailureMessage+"\n")
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
