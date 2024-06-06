package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

var (
	oAuthUrlParamValueClient = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthTokenEndpoint       = "https://auth.qovery.com/oauth/token"
)

func RefreshAccessToken() error {
	token, _ := GetRefreshToken()
	refreshToken := strings.TrimSpace(string(token))
	if refreshToken == "" {
		return errors.New("Could not reauthenticate automatically. Please, run 'qovery auth' to authenticate. ")
	}
	res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oAuthUrlParamValueClient},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
	} else {
		defer res.Body.Close()
		tokens := TokensResponse{}
		err := json.NewDecoder(res.Body).Decode(&tokens)
		if err != nil {
			return errors.New("Error authenticating in Qovery. Please, contact the #support on 'https://discord.qovery.com'. ")
		}
		expiredAt := time.Now().Local().Add(time.Second * time.Duration(30000))
		_ = SetAccessToken(AccessToken(tokens.AccessToken), expiredAt)
	}
	return nil
}

func RefreshExpiredTokenSilently() bool {
	expiration, err := GetAccessTokenExpiration()
	if err == nil && expiration.Before(time.Now()) {
		return RefreshAccessToken() == nil
	}

	return false
}
