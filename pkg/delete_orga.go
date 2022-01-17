package pkg

import (
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func DeleteOrganizationByClusterId(clusterId string, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("delete") {
		res := delete(os.Getenv("ADMIN_URL")+"/organization/"+clusterId, http.MethodDelete, dryRunDisabled)

		if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not delete organization owning cluster %s : %s. %s", clusterId, res.Status, string(result))
		} else if !dryRunDisabled {
			fmt.Println("Organization owning cluster" + clusterId + " deletable.")
		} else {
			fmt.Println("Organization owning cluster" + clusterId + " deleted.")
		}
	}
}

func delete(url string, method string, dryRunDisabled bool) *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	if dryRunDisabled {
		return nil
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(authToken)))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return res
}
