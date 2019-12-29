package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func CheckAuthenticationOrQuitWithMessage() {
	if strings.TrimSpace(GetAuthorizationToken()) == "" || strings.TrimSpace(GetAccountId()) == "" {
		fmt.Println("Are you authenticated? Consider doing 'qovery auth' to authenticate")
		os.Exit(1)
	}
}

func CheckHTTPResponse(resp *http.Response) {
	if resp.StatusCode == 401 {
		fmt.Println("Your authentication token has expired. Please re-authenticate yourself with 'qovery auth'")
		os.Exit(1)
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("Something goes wrong while requesting the Qovery API. Please try again later or contact the support (support@qovery.com)")
		os.Exit(1)
	}
}
