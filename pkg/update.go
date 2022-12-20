package pkg

import (
	"bytes"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
)

func UpdateById(clusterId string, dryRunDisabled bool, version string) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("update") {
		res := update(utils.AdminUrl+"/cluster/update/"+clusterId, http.MethodPost, dryRunDisabled, version, "", 0)

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not update cluster : %s. %s", res.Status, string(result))
		} else if !dryRunDisabled {
			fmt.Println("Cluster " + clusterId + " updatable.")
		} else {
			fmt.Println("Cluster " + clusterId + " updating.")
		}
	}
}

func UpdateAll(dryRunDisabled bool, version string, providerKind string, parallelRun int) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("update") {
		res := update(utils.AdminUrl+"/cluster/update", http.MethodPost, dryRunDisabled, version, providerKind, parallelRun)
		result, _ := io.ReadAll(res.Body)
		if strings.Contains(res.Status, "40") || strings.Contains(res.Status, "50") {
			log.Errorf("Could not update clusters : %s. %s", res.Status, string(result))
		} else {
			depl := "Deployable"
			if dryRunDisabled {
				depl = "Deploying"
			}
			log.Infof("%s clusters: %s", depl, result)
		}
	}
}

func update(url string, method string, dryRunDisabled bool, version string, providerKind string, parallelRun int) *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	content := fmt.Sprintf(`{ "metadata": { "dry_run_deploy": %t, "target_version": "%s", "provider_kind": "%s", "parallel_run": %d } }`, !dryRunDisabled, version, providerKind, parallelRun)
	body := bytes.NewBuffer([]byte(content))

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
