package util

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func GetCurrentVersion() string {
	return "0.10.1" // ci-version-check
}

func GetLatestOnlineVersionUrl() (string, error) {
	url := "https://github.com/Qovery/qovery-cli/releases/latest"
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.New("Can't reach GitHub website, please check your network connectivity")
	}
	return resp.Request.URL.Path, nil
}

func GetLatestOnlineVersionNumber() (string, error) {
	urlPath, err := GetLatestOnlineVersionUrl()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	splitUrl := strings.Split(urlPath, "/v")
	return splitUrl[len(splitUrl)-1], nil
}

func CheckAvailableNewVersion() (bool, string, string) {
	latestOnlineVersion, err := GetLatestOnlineVersionNumber()
	if err != nil {
		return false, "Error while trying to get latest version", ""
	}
	if GetCurrentVersion() != latestOnlineVersion {
		return true, fmt.Sprintf("A new version has been found %s, please upgrade it!\n"+
			"You can use your package manager or 'qovery upgrade' command.",
			latestOnlineVersion), latestOnlineVersion
	}
	return false, "You're already using the latest version", latestOnlineVersion
}
