package io

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func CheckAuthenticationOrQuitWithMessage() {
	if strings.TrimSpace(GetAuthorizationToken()) == "" || strings.TrimSpace(GetAccountId()) == "" {
		fmt.Println("Are you authenticated? Consider doing 'qovery auth' to authenticate yourself")
		os.Exit(1)
	}
}

func CheckHTTPResponse(resp *http.Response) error {
	if resp == nil {
		return errors.New("Qovery is in maintenance. Try again later or contact #support on https://discord.qovery.com")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("Your authentication token has expired. Please re-authenticate yourself with 'qovery auth'")
	} else if resp.StatusCode == http.StatusForbidden {
		return errors.New("Your account must be approved by an administrator to get access to this resource. " +
			"Please join #support on https://discord.qovery.com")
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("Something goes wrong while requesting the Qovery API. Please try again later or " +
			"contact the #support on https://discord.qovery.com")
	}

	return nil
}
