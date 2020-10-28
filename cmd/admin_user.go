package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var adminUserCmd = &cobra.Command{Use: "user"}

func init() {
	adminCmd.AddCommand(adminUserCmd)
}

func getTokens() (string, string) {
	authorizationToken := io.GetAuthorizationToken()
	adminToken := io.GetAdminToken()
	if authorizationToken == "" {
		log.Fatal("Authorization token not found. Use 'qovery auth' to sign in first. ")
	}
	if adminToken == "" {
		log.Fatal("Admin token not found. Use 'qovery auth' to sign as admin user first. ")
	}
	return authorizationToken, adminToken
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
