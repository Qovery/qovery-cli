package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"qovery-cli/io"
	"strings"
)

var adminUserCmd = &cobra.Command{Use: "user"}

func init() {
	adminCmd.AddCommand(adminUserCmd)
}

func getTokens() (string, string) {
	authorizationToken := io.GetAuthorizationToken()
	if authorizationToken == "" {
		log.Fatal("Authorization token not found. Use 'qovery auth' to sign in first. ")
	}

	adminToken := getAdminToken(authorizationToken)
	if adminToken == "" {
		log.Fatal("Admin token not found. Use 'qovery auth' to sign as admin user first. ")
	}

	return authorizationToken, adminToken
}

func getAdminToken(authorizationToken string) string {
	type Response struct {
		AccessToken string `json:"access_token"`
	}

	req, _ := http.NewRequest(http.MethodGet, io.RootURL+"/admin/management-token", nil)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(authorizationToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var parsedResBody Response
	err = json.Unmarshal(body, &parsedResBody)
	if err != nil {
		log.Fatal(err)
	}

	return strings.TrimSpace(parsedResBody.AccessToken)
}

func prepareUserMetadataPayload(user string) *bytes.Buffer {
	payload := payload{UserMetadata: userMetadata{Rsub: user}}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error preparing request body. ")
	}

	return bytes.NewBuffer(jsonPayload)
}

func prepareEmptyUserMetadataPayload() *bytes.Buffer {
	payload := clearUserMetadataPayload{UserMetadata: emptyUserMetadata{}}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error preparing request body. ")
	}

	return bytes.NewBuffer(jsonPayload)
}

func getSubClaim(authorizationToken string) string {
	parsed, _, err := new(jwt.Parser).ParseUnverified(authorizationToken, jwt.MapClaims{})
	if err != nil {
		log.Fatal(err)
	}

	var sub string
	if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
		sub = fmt.Sprintf("%s", claims["sub"])
	} else {
		log.Fatal(err)
	}

	return sub
}
