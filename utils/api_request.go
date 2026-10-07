package utils

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/variable"
)

// NewAPIRequest builds an authenticated request to the public API, for endpoints the typed client lacks.
func NewAPIRequest(method string, path string, body io.Reader, allowNoOrganization bool) (*http.Request, error) {
	req, err := http.NewRequest(method, GetAPIBaseURL()+"/"+strings.TrimLeft(path, "/"), body)
	if err != nil {
		return nil, err
	}

	tokenType, token, err := GetAccessToken(allowNoOrganization)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("User-Agent", "CLI "+Version)
	return req, nil
}

// DoAPIRequest sends req with the typed client's timeout, dumping request and response under --verbose.
func DoAPIRequest(req *http.Request) (*http.Response, error) {
	if variable.Verbose {
		if dump, err := dumpRequestRedacted(req); err == nil {
			log.Printf("\n%s\n", dump)
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if variable.Verbose {
		if dump, err := httputil.DumpResponse(resp, true); err == nil {
			log.Printf("\n%s\n", string(dump))
		}
	}
	return resp, nil
}

func dumpRequestRedacted(req *http.Request) (string, error) {
	auth := req.Header.Get("Authorization")
	if auth != "" {
		req.Header.Set("Authorization", redactAuthorization(auth))
		defer req.Header.Set("Authorization", auth)
	}

	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return "", err
	}
	return string(dump), nil
}

func redactAuthorization(value string) string {
	if scheme, _, found := strings.Cut(value, " "); found {
		return scheme + " ***"
	}
	return "***"
}
