package pkg

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/browser"

	"github.com/qovery/qovery-cli/utils"
)

const (
	httpAuthPort     = 10999
	qoveryConsoleUrl = "https://console.qovery.com"
	oAuthQoveryUrl   = "https://auth.qovery.com/login?code_challenge_method=S256&scope=%s&client=%s&protocol=oauth2&response_type=%s&audience=%s&redirect_uri=%s&code_challenge=%s"
)

var (
	// Delay before the callback server shuts down, so the browser gets its response.
	authShutdownDelay = time.Second

	oAuthUrlParamValueClient         = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthUrlParamValueHeadlessClient = "f9drkTNpxsEw2VU2PVDrxhyT3vVuFT0Y"
	oAuthUrlParamValueAudience       = "https://core.qovery.com"
	oAuthUrlParamValueResponseType   = "code"
	oAuthUrlParamValueScopes         = "offline_access openid profile email"
	oAuthUrlParamValueRedirect       = "http://localhost:" + strconv.Itoa(httpAuthPort) + "/authorization"
	oAuthTokenEndpoint               = "https://auth.qovery.com/oauth/token"

	authSuccessMessage = "Authentication successful, you'll be redirected to Qovery console. If it's not the case, click on this link: "
	authFailureMessage = "Authentication failed. Run 'qovery auth' again, or contact #support on https://discord.qovery.com."

	// Upper bound for the graceful shutdown. It frees the port; it does not
	// decide when the failure response is complete, see awaitFailureDelivered.
	authShutdownTimeout = 5 * time.Second

	// exitProcess is replaceable so tests can observe an authentication failure.
	exitProcess = os.Exit
)

type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    uint   `json:"expires_in"`
}
type DeviceFlowParameters struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

func DoRequestUserToAuthenticate(headless bool, skipVersionCheck bool) {
	if !skipVersionCheck {
		available, message, _ := CheckAvailableNewVersion()
		if available {
			fmt.Println(message)
		}
	}
	if headless {
		runHeadlessFlow()
		return
	}

	verifier := createCodeVerifier()
	challenge, err := createCodeChallengeS256(verifier)
	if err != nil {
		utils.PrintlnError(errors.New("can not create authorization code challenge. Please contact the #support at 'https://discord.qovery.com'. "))
		os.Exit(0)
	}
	// Listen before opening the browser so a busy port is reported instead of
	// leaving the user waiting on a callback that can never arrive.
	srv := newAuthorizationServer(verifier, storeTokens)
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("can not listen on %s for the authentication callback, is another 'qovery auth' running? %w", srv.Addr, err))
		os.Exit(1)
	}

	// TODO link to web auth
	_ = browser.OpenURL(fmt.Sprintf(oAuthQoveryUrl, url.QueryEscape(oAuthUrlParamValueScopes), oAuthUrlParamValueClient, url.QueryEscape(oAuthUrlParamValueResponseType),
		url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect), challenge))

	fmt.Println("\nOpening your browser, waiting for your authentication... ")

	serveAuthorization(srv, listener)
}

// authorizationServer is the callback server of one authentication attempt.
type authorizationServer struct {
	*http.Server
	failed atomic.Bool

	// shutdownTimeout is read once at construction, so the package variable is
	// not touched from server goroutines.
	shutdownTimeout time.Duration

	// failedConn is the connection that carries the failure response.
	// delivered closes once that connection is idle again after the failing
	// handler: the http server only gets there after the response is completely
	// written. Closed and hijacked connections prove nothing about delivery.
	failMu        sync.Mutex
	failedConn    net.Conn
	delivered     chan struct{}
	deliveredOnce sync.Once
}

type connContextKey struct{}

func (s *authorizationServer) markDelivered() {
	s.deliveredOnce.Do(func() { close(s.delivered) })
}

// serveAuthorization serves until the attempt ends. Serve returns as soon as
// Shutdown starts, and Shutdown closes the listener first, so the fixed port is
// free again when this function returns. On failure the process exits here, not
// in the handler, and only once the failure response is confirmed completely
// written: the browser must not get it truncated. If that never gets confirmed
// (connection lost mid-write), the CLI keeps waiting instead of exiting.
func serveAuthorization(srv *authorizationServer, listener net.Listener) {
	_ = srv.Serve(listener)
	srv.shutdown()
	if srv.failed.Load() {
		srv.awaitFailureDelivered()
		exitProcess(1)
	}
}

// shutdown stops the server and waits, within authShutdownTimeout, for in-flight
// requests to finish. It is safe to call more than once. A timeout here does not
// mean the failure response is complete: see awaitFailureDelivered.
func (s *authorizationServer) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		utils.PrintlnError(err)
	}
}

// awaitFailureDelivered blocks until the failure response is confirmed
// completely written. There is no timeout on purpose: exiting earlier could
// truncate the response the user is about to read.
func (s *authorizationServer) awaitFailureDelivered() {
	<-s.delivered
}

// fail answers the browser with the failure message and ends the attempt. The
// handler only returns: exiting here would kill the process before the response
// is complete (a flushed response is still chunked and unterminated).
func (s *authorizationServer) fail(writer http.ResponseWriter, request *http.Request, status int) {
	conn, _ := request.Context().Value(connContextKey{}).(net.Conn)
	s.failMu.Lock()
	s.failedConn = conn
	s.failMu.Unlock()
	if conn == nil {
		s.markDelivered() // not served through serveAuthorization: nothing to wait for
	}

	http.Error(writer, authFailureMessage, status)
	utils.PrintlnError(errors.New("authentication unsuccessful. Try again later or contact #support on 'https://discord.qovery.com'. "))
	if s.failed.CompareAndSwap(false, true) {
		// Shutdown disables keep-alive, so a response finished during it closes
		// the connection without ever going idle and delivery could not be
		// confirmed. Shut down only once it is.
		go func() {
			s.awaitFailureDelivered()
			s.shutdown()
		}()
	}
}

func storeTokens(tokens TokensResponse) {
	expiredAt := time.Now().Local().Add(time.Duration(tokens.ExpiresIn-60) * time.Second)
	_ = utils.SetAccessToken(utils.AccessToken(tokens.AccessToken), expiredAt, utils.RefreshToken(tokens.RefreshToken))
}

// authorizationPage is the callback page. The element is named statusElement
// because a global status resolves to window.status, which is string-valued:
// assigning the DOM element to it would store a string and lose the element.
func authorizationPage(port int) string {
	return fmt.Sprintf(`<p id="status">Authenticating...</p>
<script type="text/javascript" charset="utf-8">
	var statusElement = document.getElementById("status");
	var code = new URLSearchParams(window.location.search).get("code") || "";
	var xmlHttp = new XMLHttpRequest();
	xmlHttp.open("GET", "http://localhost:%d/authorization/valid?code=" + encodeURIComponent(code), true);
	xmlHttp.onload = function () {
		statusElement.textContent = xmlHttp.responseText;
		if (xmlHttp.status === 200) {
			var link = document.createElement("a");
			link.href = %q;
			link.textContent = %q;
			statusElement.appendChild(link);
			window.setTimeout(function () { window.location = %q; }, 2000);
		}
	};
	xmlHttp.onerror = function () {
		statusElement.textContent = "Authentication failed, the Qovery CLI could not be reached. Run 'qovery auth' again.";
	};
	xmlHttp.send(null);
</script>`, port, qoveryConsoleUrl, qoveryConsoleUrl, qoveryConsoleUrl)
}

// newAuthorizationServer builds a server with its own mux for one interactive
// authentication attempt. The handlers must not be registered on
// http.DefaultServeMux: a second attempt in the same process (re-auth after a
// 401) would panic on the duplicate "/authorization" pattern. The PKCE verifier
// is captured per attempt.
func newAuthorizationServer(verifier string, onTokens func(TokensResponse)) *authorizationServer {
	srv := &authorizationServer{
		Server:    &http.Server{Addr: fmt.Sprintf("localhost:%d", httpAuthPort)},
		delivered: make(chan struct{}),

		shutdownTimeout: authShutdownTimeout,
	}
	srv.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, connContextKey{}, conn)
	}
	srv.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateIdle:
			srv.failMu.Lock()
			isFailed := conn == srv.failedConn
			srv.failMu.Unlock()
			if isFailed {
				srv.markDelivered()
			}
		case http.StateNew, http.StateActive, http.StateHijacked, http.StateClosed:
		}
	}
	mux := http.NewServeMux()

	// The page only shows what /authorization/valid answers: the success message
	// is not part of it, so a failed token exchange can never look like a success.
	mux.HandleFunc("/authorization", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(authorizationPage(httpAuthPort)))
	})

	// The callback is one-shot. A repeat (page reload, double request) must not
	// replay the consumed code: it gets the first success again, whatever it
	// carries. The lock also makes a concurrent repeat wait for the first exchange.
	var (
		mu   sync.Mutex
		done bool
	)
	mux.HandleFunc("/authorization/valid", func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if done {
			_, _ = writer.Write([]byte(authSuccessMessage))
			return
		}

		// The attempt already failed and is shutting down: do not exchange again.
		if srv.failed.Load() {
			http.Error(writer, authFailureMessage, http.StatusUnauthorized)
			return
		}

		codes := request.URL.Query()["code"]
		if len(codes) == 0 || codes[0] == "" {
			srv.fail(writer, request, http.StatusBadRequest)
			return
		}
		tokens, err := exchangeAuthorizationCode(verifier, codes[0])
		if err != nil {
			// A rejected or stale code must not be stored nor reported as a success.
			srv.fail(writer, request, http.StatusUnauthorized)
			return
		}
		done = true
		onTokens(tokens)
		utils.PrintlnInfo("Success!")
		_, _ = writer.Write([]byte(authSuccessMessage))

		delay := authShutdownDelay
		go func() {
			time.Sleep(delay)
			srv.shutdown()
		}()
	})

	srv.Handler = mux
	return srv
}

// exchangeAuthorizationCode trades the authorization code for tokens. It fails
// on a non-200 answer or an empty access token, which is how the token endpoint
// reports a rejected or already used code.
func exchangeAuthorizationCode(verifier string, code string) (TokensResponse, error) {
	res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {oAuthUrlParamValueClient},
		"code":          {code},
		"redirect_uri":  {oAuthUrlParamValueRedirect},
		"code_verifier": {verifier},
	})
	if err != nil {
		return TokensResponse{}, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return TokensResponse{}, fmt.Errorf("token endpoint answered %d", res.StatusCode)
	}
	// Read the whole body: Unmarshal rejects trailing data after the first JSON
	// value, which a streaming Decode would accept.
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		return TokensResponse{}, err
	}
	tokens := TokensResponse{}
	if err := json.Unmarshal(payload, &tokens); err != nil {
		return TokensResponse{}, err
	}
	if tokens.AccessToken == "" {
		return TokensResponse{}, errors.New("token endpoint returned an empty access token")
	}
	return tokens, nil
}

func createCodeVerifier() string {
	length := 64
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[i] = byte(r.Intn(255))
	}
	return encode(b)
}

func createCodeChallengeS256(verifier string) (string, error) {
	h := sha256.New()
	_, err := h.Write([]byte(verifier))
	if err != nil {
		return "", err
	}
	return encode(h.Sum(nil)), nil
}

func encode(msg []byte) string {
	encoded := base64.StdEncoding.EncodeToString(msg)
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "=", "")
	return encoded
}

func runHeadlessFlow() {
	parameters := deviceFlowParameters()
	requestDeviceActivationWith(parameters)
	start := time.Now()

	fmt.Println("Waiting for code confirmation...")

	for time.Since(start).Seconds() < float64(parameters.ExpiresIn) {
		time.Sleep(time.Second * time.Duration(parameters.Interval))
		tokens, err := getTokensWith(parameters)

		if err == nil {
			storeTokens(tokens)
			utils.PrintlnInfo("Success!")
			return
		}
	}

	fmt.Println("Code has expired! ")
	os.Exit(0)
}

func deviceFlowParameters() DeviceFlowParameters {
	endpoint := "https://auth.qovery.com/oauth/device/code"
	payload := strings.NewReader(fmt.Sprintf("client_id=%s&scope=%s&audience=%s&redirect_uri=%s", url.QueryEscape(oAuthUrlParamValueHeadlessClient), url.QueryEscape(oAuthUrlParamValueScopes), url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect)))
	req, err := http.NewRequest("POST", endpoint, payload)

	if err != nil {
		printContactSupportMessage("Error forming device code request. ")
		os.Exit(0)
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		printContactSupportMessage("Error getting device code. ")
		os.Exit(0)
	}

	if res.StatusCode == 200 {
		defer func() {
			if err := res.Body.Close(); err != nil {
				utils.PrintlnError(fmt.Errorf("error closing response body: %w", err))
			}
		}()

		parameters := DeviceFlowParameters{}
		err = json.NewDecoder(res.Body).Decode(&parameters)

		if err != nil {
			printContactSupportMessage("Error parsing device code response. ")
			os.Exit(0)
		}

		return parameters
	} else {
		printContactSupportMessage("Error getting device code. ")
		os.Exit(0)
		return DeviceFlowParameters{}
	}
}

func printContactSupportMessage(msg string) {
	fmt.Println(msg)
	fmt.Println("Please contact the #support at 'https://discord.qovery.com'. ")
}

func requestDeviceActivationWith(params DeviceFlowParameters) {
	fmt.Println("Please, open browser @ " + params.VerificationUri + " using any device and enter " + params.UserCode + " code. ")
}

func getTokensWith(params DeviceFlowParameters) (TokensResponse, error) {
	endpoint := "https://auth.qovery.com/oauth/token"
	payload := strings.NewReader("grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code=" + params.DeviceCode + "&client_id=" + oAuthUrlParamValueHeadlessClient)
	req, err := http.NewRequest("POST", endpoint, payload)

	if err != nil {
		printContactSupportMessage("Error forming get access token request. ")
		os.Exit(0)
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		printContactSupportMessage("Error pooling access token. ")
		os.Exit(0)
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			utils.PrintlnError(fmt.Errorf("error closing response body: %w", err))
		}
	}()

	if res.StatusCode == 200 {
		tokens := TokensResponse{}
		err = json.NewDecoder(res.Body).Decode(&tokens)
		return tokens, err
	} else {
		return TokensResponse{}, errors.New("could not fetch tokens")
	}
}

type QoveryClientApiRequest[T any] func(needToRefetchClient bool) (*T, *http.Response, error)

// RetryQoveryClientApiRequestOnUnauthorized To be able to ask for re-auth when first attempt leads to unauthorized
func RetryQoveryClientApiRequestOnUnauthorized[T any](request QoveryClientApiRequest[T]) (*T, *http.Response, error) {
	qoveryStruct, response, err := request(false)
	if response != nil && response.StatusCode == http.StatusUnauthorized {
		utils.Println("Needs to re-authenticate as the response is UNAUTHORIZED (401)")
		DoRequestUserToAuthenticate(false, true)
		qoveryStruct, response, err = request(true)
	}
	return qoveryStruct, response, err
}
