package pkg

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// stubTokenEndpoint records the code_verifier of every token exchange.
func stubTokenEndpoint(t *testing.T) *[]string {
	t.Helper()
	var mu sync.Mutex
	verifiers := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		verifiers = append(verifiers, r.PostForm.Get("code_verifier")+"|"+r.PostForm.Get("code"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","expires_in":3600}`))
	}))
	t.Cleanup(ts.Close)

	prevEndpoint, prevDelay := oAuthTokenEndpoint, authShutdownDelay
	oAuthTokenEndpoint = ts.URL
	authShutdownDelay = 0
	t.Cleanup(func() { oAuthTokenEndpoint, authShutdownDelay = prevEndpoint, prevDelay })
	return &verifiers
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
	if len(*verifiers) != len(want) {
		t.Fatalf("token exchanges = %v, want %v", *verifiers, want)
	}
	for i := range want {
		if (*verifiers)[i] != want[i] {
			t.Errorf("exchange %d = %q, want %q", i, (*verifiers)[i], want[i])
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
	for i := range want {
		if (*verifiers)[i] != want[i] {
			t.Errorf("exchange %d = %q, want %q", i, (*verifiers)[i], want[i])
		}
	}
	if len(*verifiers) != 2 {
		t.Errorf("token exchanges = %v, want %v", *verifiers, want)
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
