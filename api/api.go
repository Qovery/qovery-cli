package api

import (
	"fmt"
	"os"
	"strings"
)

func CheckAuthenticationOrQuitWithMessage() {
	if strings.TrimSpace(GetAuthorizationToken()) == "" || strings.TrimSpace(GetAccountId()) == "" {
		fmt.Println("Are you authenticated? Consider doing 'qovery auth' to authenticate")
		os.Exit(1)
	}
}
