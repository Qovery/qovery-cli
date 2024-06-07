package utils

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokensResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   uint   `json:"expires_in"`
}

var (
	oAuthUrlParamValueClient = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthTokenEndpoint       = "https://auth.qovery.com/oauth/token"
)

func RefreshAccessToken(token RefreshToken) (AccessToken, error) {
	refreshToken := strings.TrimSpace(string(token))
	if refreshToken == "" {
		return "", errors.New("Could not reauthenticate automatically. Please, run 'qovery auth' to authenticate. ")
	}
	res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oAuthUrlParamValueClient},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return "", errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	tokens := TokensResponse{}
	err = json.NewDecoder(res.Body).Decode(&tokens)
	if err != nil {
		return "", errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	}
	expiredAt := time.Now().Local().Add(time.Duration(tokens.ExpiresIn-60) * time.Second)
	accessToken := AccessToken(tokens.AccessToken)
	// We dont have refreshToken rotation enabled, we should, ...
	// So the response does not contain a new refresh token to use. We keep the old one
	_ = SetAccessToken(accessToken, expiredAt, token)

	return accessToken, nil
}
