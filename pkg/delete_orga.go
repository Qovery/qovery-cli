package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

func DeleteOrganizationByClusterId(clusterId string, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("delete") {
		res := httpDelete(utils.AdminUrl+"/organization?clusterId="+clusterId, http.MethodDelete, dryRunDisabled)

		if !dryRunDisabled {
			fmt.Println("Organization owning cluster" + clusterId + " deletable.")
		} else if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not delete organization owning cluster %s : %s. %s", clusterId, res.Status, string(result))
		} else {
			fmt.Println("Organization owning cluster" + clusterId + " deleted.")
		}
	}
}

func httpDelete(url string, method string, dryRunDisabled bool) *http.Response {
	return deleteWithBody(url, method, dryRunDisabled, nil)
}

func deleteWithBody(url string, method string, dryRunDisabled bool, body io.Reader) *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	if !dryRunDisabled {
		return nil
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return res
}
