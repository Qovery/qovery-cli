package pkg

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryQoveryClientApiRequestOnUnauthorized(t *testing.T) {
	// a token from the environment is returned as is by utils.GetAccessToken, without any network call
	setupValidAccessToken := func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("QOVERY_CLI_ACCESS_TOKEN", "fake-token")
	}

	t.Run("Should not retry when first request is authorized", func(t *testing.T) {
		// given
		setupValidAccessToken(t)
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			result := "ok"
			return &result, &http.Response{StatusCode: http.StatusOK}, nil
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false}, calls)
	})

	t.Run("Should refetch client silently without interactive auth when token has expired", func(t *testing.T) {
		// given
		setupValidAccessToken(t)
		var calls []bool
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if !needToRefetchClient {
				return nil, &http.Response{StatusCode: http.StatusUnauthorized}, nil
			}
			result := "ok"
			return &result, &http.Response{StatusCode: http.StatusOK}, nil
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, "ok", *result)
		assert.Equal(t, []bool{false, true}, calls)
	})

	t.Run("Should return transport error without interactive auth when token refresh succeeded", func(t *testing.T) {
		// given
		setupValidAccessToken(t)
		var calls []bool
		transportErr := errors.New("dial tcp: i/o timeout")
		request := func(needToRefetchClient bool) (*string, *http.Response, error) {
			calls = append(calls, needToRefetchClient)
			if !needToRefetchClient {
				return nil, &http.Response{StatusCode: http.StatusUnauthorized}, nil
			}
			return nil, nil, transportErr
		}

		// when
		result, response, err := RetryQoveryClientApiRequestOnUnauthorized(request)

		// then
		assert.ErrorIs(t, err, transportErr)
		assert.Nil(t, response)
		assert.Nil(t, result)
		assert.Equal(t, []bool{false, true}, calls)
	})
}
