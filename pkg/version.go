package pkg

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	semver "github.com/Masterminds/semver/v3"

	"github.com/qovery/qovery-cli/utils"
)

// wil be replaced by CI by the latest git tag
var Version = "unknown"

func GetCurrentVersion() (*semver.Version, error) {
	version, err := semver.NewVersion(Version)
	if err != nil {
		return nil, fmt.Errorf("error trying to get semver from raw string `%s`, error: `%w`", Version, err)
	}

	return version, nil
}

func GetLatestOnlineVersionUrl() (string, error) {
	url := "https://github.com/Qovery/qovery-cli/releases/latest"
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.New("can't reach Github, please check your network connectivity")
	}

	return resp.Request.URL.Path, nil
}

func GetLatestOnlineVersionNumber() (*semver.Version, error) {
	urlPath, err := GetLatestOnlineVersionUrl()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}
	splitUrl := strings.Split(urlPath, "/v")

	version, err := semver.NewVersion(splitUrl[len(splitUrl)-1])
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	return version, nil
}

func CheckAvailableNewVersion() (bool, string, *semver.Version) {
	latestOnlineVersion, err := GetLatestOnlineVersionNumber()
	if err != nil {
		return false, "Error while trying to get the latest version. ", nil
	}
	currentVersion, err := GetCurrentVersion()
	if err != nil {
		return false, fmt.Sprintf("Error while trying to get the current version, mostlikely current version `%s` is not a valid semver string, error: `%s`", Version, err), nil
	}
	if latestOnlineVersion.GreaterThan(currentVersion) {
		return true, fmt.Sprintf("A new version has been found %s, please upgrade it. \n"+
			"You can use your package manager or 'qovery upgrade' command. ",
			latestOnlineVersion), latestOnlineVersion
	}
	return false, "You're already using the latest version. ", latestOnlineVersion
}
