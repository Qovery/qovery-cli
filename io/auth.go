package io

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pkg/browser"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	httpAuthPort   = 10999
	oAuthQoveryUrl = "https://auth.qovery.com/login?code_challenge_method=S256&scope=%s&client=%s&protocol=oauth2&response_type=%s&audience=%s&redirect_uri=%s&code_challenge=%s"
)

var (
	oAuthUrlParamValueClient       = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthUrlParamValueAudience     = "https://core.qovery.com"
	oAuthUrlParamValueResponseType = "code"
	oAuthUrlParamValueScopes       = "offline_access openid profile email"
	oAuthUrlParamValueRedirect     = "http://localhost:" + strconv.Itoa(httpAuthPort) + "/authorization"
	oAuthTokenEndpoint             = "https://auth.qovery.com/oauth/token"
)

type TokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func DoRequestUserToAuthenticate() {
	available, message, _ := CheckAvailableNewVersion()
	if available {
		fmt.Println(message)
	}

	verifier := createCodeVerifier()
	challenge, err := createCodeChallengeS256(verifier)
	if err != nil {
		fmt.Println("Can not create authorization code challenge. Please contact the #support at 'https://discord.qovery.com'.")
		os.Exit(1)
	}
	// TODO link to web auth
	_ = browser.OpenURL(fmt.Sprintf(oAuthQoveryUrl, url.QueryEscape(oAuthUrlParamValueScopes), oAuthUrlParamValueClient, url.QueryEscape(oAuthUrlParamValueResponseType),
		url.QueryEscape(oAuthUrlParamValueAudience), url.QueryEscape(oAuthUrlParamValueRedirect), challenge))

	fmt.Println("\nOpening your browser, waiting for your authentication...")

	srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", httpAuthPort)}

	http.HandleFunc("/authorization", func(writer http.ResponseWriter, request *http.Request) {
		js := fmt.Sprintf(`<script type="text/javascript" charset="utf-8">
				var hash = window.location.search.split("=")[1].split("&")[0];
				var xmlHttp = new XMLHttpRequest();
				xmlHttp.open("GET", "http://localhost:%d/authorization/valid?code=" + hash, false);
				xmlHttp.send(null);
				xmlHttp.responseText;
             </script>`, httpAuthPort)

		_, _ = writer.Write([]byte(js))
		_, _ = writer.Write([]byte("Authentication successful. You can close this window."))
	})

	http.HandleFunc("/authorization/valid", func(writer http.ResponseWriter, request *http.Request) {
		code := request.URL.Query()["code"][0]
		res, err := http.PostForm(oAuthTokenEndpoint, url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {oAuthUrlParamValueClient},
			"code":          {code},
			"redirect_uri":  {oAuthUrlParamValueRedirect},
			"code_verifier": {verifier},
		})

		if err != nil {
			println("Authentication unsuccessful. Try again later or contact #support on 'https://discord.qovery.com'. ")
			os.Exit(1)
		} else {
			defer res.Body.Close()
			tokens := TokensResponse{}
			err := json.NewDecoder(res.Body).Decode(&tokens)
			if err != nil {
				println("Authentication unsuccessful. Try again later or contact #support on 'https://discord.qovery.com'. ")
				os.Exit(1)
			}
			expiredAt := time.Now().Local().Add(time.Second * time.Duration(30000))
			SetAuthorizationToken(tokens.AccessToken)
			SetRefreshToken(tokens.RefreshToken)
			SetAuthorizationTokenExpiration(expiredAt)
			accountId := GetAccount().Id
			if accountId != "" {
				SetAccountId(accountId)
				fmt.Println("Authentication successful!")
			}
		}

		go func() {
			time.Sleep(time.Second)
			if err := srv.Shutdown(context.TODO()); err != nil {
				log.Printf("fail to shudown http server: %s", err.Error())
			}
		}()
	})

	_ = srv.ListenAndServe()
}

func RefreshAccessToken() error {
	refreshToken := strings.TrimSpace(GetRefreshToken())
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
		SetAuthorizationToken(tokens.AccessToken)
		SetAuthorizationTokenExpiration(expiredAt)
		accountId := GetAccount().Id
		if accountId != "" {
			SetAccountId(accountId)
		} else {
			return errors.New("Could not reauthenticate automatically. Please, run 'qovery auth' to authenticate. ")
		}
	}
	return nil
}

func RefreshExpiredTokenSilently() {
	refreshToken := strings.TrimSpace(GetRefreshToken())
	expiration, err := GetAuthorizationTokenExpiration()

	if err == nil && expiration.Before(time.Now()) && refreshToken != "" {
		err := RefreshAccessToken()
		if err != nil {
			// we don't care
		}
	}
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
	encoded = strings.Replace(encoded, "+", "-", -1)
	encoded = strings.Replace(encoded, "/", "_", -1)
	encoded = strings.Replace(encoded, "=", "", -1)
	return encoded
}
