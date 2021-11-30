package pkg

import (
	"bytes"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func UpdateById(clusterId string, dryRunDisabled bool, version string){
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("update") {
		res := update(os.Getenv("ADMIN_URL") + "/cluster/update/" + clusterId, http.MethodPost, dryRunDisabled, version)

		if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not update cluster : %s. %s", res.Status, string(result) )
		} else {
			fmt.Println("Cluster " + clusterId + " updating.")
		}
	}
}

func UpdateAll(dryRunDisabled bool, version string) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("update") {
		res := update(os.Getenv("ADMIN_URL") + "/cluster/update", http.MethodPost, dryRunDisabled, version)

		if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not update clusters : %s. %s", res.Status, string(result) )
		} else {
			fmt.Println("Clusters updating.")
		}
	}
}

func update(url string, method string, dryRunDisabled bool, version string) *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	var body *bytes.Buffer

	if dryRunDisabled {
		body = bytes.NewBuffer([]byte(fmt.Sprintf(`{ "metadata": { "dry_run_deploy": true, "target_version": "%s" } }`, version)))
	}

	req, err  := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer " + strings.TrimSpace(string(authToken)))
	if dryRunDisabled {
		req.Header.Set("Content-Type", "application/json")
	}


	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return res
}
