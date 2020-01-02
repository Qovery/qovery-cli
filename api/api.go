package api

import (
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

func CheckHTTPResponse(resp *http.Response) {
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Println("Your authentication token has expired. Please re-authenticate yourself with 'qovery auth'")
		os.Exit(1)
	} else if resp.StatusCode == http.StatusForbidden {
		fmt.Println("Your account must be approved by an administrator to get access to this resource. Please contact support@qovery.com or through intercom on qovery.com")
		os.Exit(1)
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println(errorUnknownError)
		os.Exit(1)
	}
}
