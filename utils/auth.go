package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokensResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   uint   `json:"expires_in"`
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

var (
	// oAuthUrlParamValueClient is the client of the browser flow ('qovery auth')
	oAuthUrlParamValueClient = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthTokenEndpoint       = "https://auth.qovery.com/oauth/token"
)

// oAuthClientOf returns the client that issued the access token, read from its azp claim.
// 'qovery auth' and 'qovery auth --headless' sign in with two different clients, and a refresh token is only accepted
// with the client it was issued to. Falls back to the browser flow client when the token does not carry the claim.
func oAuthClientOf(token AccessToken) string {
	claims := jwt.MapClaims{}
	// The token is only read to pick the client: the authentication server is the one validating the refresh
	_, _, _ = jwt.NewParser().ParseUnverified(strings.TrimSpace(string(token)), claims)
	if client, ok := claims["azp"].(string); ok && client != "" {
		return client
	}
	return oAuthUrlParamValueClient
}

// RefreshAccessToken gets a new access token from the refresh token and stores it.
// clientId has to be the client the refresh token was issued to, see oAuthClientOf.
func RefreshAccessToken(token RefreshToken, clientId string) (AccessToken, error) {
	refreshToken := strings.TrimSpace(string(token))
	if refreshToken == "" {
		return "", fmt.Errorf("could not reauthenticate automatically. Please, %w. ", ErrSignInRequired)
	}
	res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientId},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return "", errors.New("error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		// The refresh token is rejected (revoked, expired, issued to another client): only a new sign-in can fix it
		reason := tokenErrorResponse{}
		_ = json.NewDecoder(res.Body).Decode(&reason)
		if reason.ErrorDescription == "" {
			reason.ErrorDescription = res.Status
		}
		return "", fmt.Errorf("could not reauthenticate automatically: %s Please, %w. ", reason.ErrorDescription, ErrSignInRequired)
	default:
		return "", fmt.Errorf("could not refresh the access token: the authentication server answered %s", res.Status)
	}

	tokens := TokensResponse{}
	err = json.NewDecoder(res.Body).Decode(&tokens)
	if err != nil {
		return "", errors.New("error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	}
	if tokens.AccessToken == "" {
		return "", errors.New("could not refresh the access token: the authentication server returned no access token")
	}
	expiredAt := time.Now().Local().Add(time.Duration(tokens.ExpiresIn-60) * time.Second)
	accessToken := AccessToken(tokens.AccessToken)
	// We dont have refreshToken rotation enabled, we should, ...
	// So the response does not contain a new refresh token to use. We keep the old one
	_ = SetAccessToken(accessToken, expiredAt, token)

	return accessToken, nil
}
