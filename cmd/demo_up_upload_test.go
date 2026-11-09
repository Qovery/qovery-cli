package cmd

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostDemoDebugLogRetriesOnceOnServerErrors(t *testing.T) {
	tests := []struct {
		name          string
		statuses      []int
		wantRequests  int
		wantErr       bool
		wantAuthError bool
	}{
		{name: "stored at once", statuses: []int{http.StatusOK}, wantRequests: 1},
		{name: "stored on retry", statuses: []int{http.StatusInternalServerError, http.StatusOK}, wantRequests: 2},
		{name: "server error twice", statuses: []int{http.StatusBadGateway, http.StatusServiceUnavailable}, wantRequests: 2, wantErr: true},
		{name: "client error is not retried", statuses: []int{http.StatusBadRequest}, wantRequests: 1, wantErr: true},
		{name: "unauthorized", statuses: []int{http.StatusUnauthorized}, wantRequests: 1, wantErr: true, wantAuthError: true},
		{name: "forbidden", statuses: []int{http.StatusForbidden}, wantRequests: 1, wantErr: true, wantAuthError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var bodies []string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body, _ := io.ReadAll(request.Body)
				bodies = append(bodies, string(body))
				assert.Equal(t, "Token abc", request.Header.Get("Authorization"))
				assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
				writer.WriteHeader(test.statuses[len(bodies)-1])
			}))
			defer server.Close()

			err := postDemoDebugLog(server.Client(), server.URL, "Token abc", []byte(`{"content":"log"}`), 0)

			assert.Len(t, bodies, test.wantRequests)
			for _, body := range bodies {
				assert.Equal(t, `{"content":"log"}`, body)
			}
			if !test.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, test.wantAuthError, isDemoDebugLogAuthFailure(err))
		})
	}
}

func TestPostDemoDebugLogRetriesOnceOnANetworkError(t *testing.T) {
	transport := &failingDemoTransport{}

	err := postDemoDebugLog(&http.Client{Transport: transport}, "http://qovery.invalid/admin/demoDebugLog", "Token abc", []byte("{}"), 0)

	require.Error(t, err)
	assert.Equal(t, 2, transport.attempts)
	assert.False(t, isDemoDebugLogAuthFailure(err))
}

func TestPostDemoDebugLogDoesNotRetryAnInvalidEndpoint(t *testing.T) {
	transport := &failingDemoTransport{}
	startedAt := time.Now()

	err := postDemoDebugLog(&http.Client{Transport: transport}, "://no-scheme", "Token abc", []byte("{}"), time.Minute)

	require.Error(t, err)
	assert.Zero(t, transport.attempts)
	assert.Less(t, time.Since(startedAt), time.Minute)
}

type failingDemoTransport struct {
	attempts int
}

func (f *failingDemoTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.attempts++
	return nil, errors.New("connection refused")
}
