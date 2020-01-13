package api

import (
	"fmt"
	"os"
	"strings"
)

func CheckAuthenticationOrQuitWithMessage() {
	if strings.TrimSpace(GetAuthorizationToken()) == "" || strings.TrimSpace(GetAccountId()) == "" {
		fmt.Println("Are you authenticated? Consider doing 'qovery auth' to authenticate yourself")
		os.Exit(1)
	}
}

func printDebug(log string, values ...interface{}) {
	fmt.Println("DEBUG: " + fmt.Sprintf(log, values...))
}
