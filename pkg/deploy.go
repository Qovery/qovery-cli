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

func DeployById(clusterId string, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("deployment") {
		res := deploy(utils.AdminUrl+"/cluster/deploy/"+clusterId, http.MethodPost, dryRunDisabled)

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not deploy cluster : %s. %s", res.Status, string(result))
		} else if !dryRunDisabled {
			fmt.Println("Cluster " + clusterId + " deployable.")
		} else {
			fmt.Println("Cluster " + clusterId + " deploying.")
		}
	}
}

func DeployAll(dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("deployment") {
		res := deploy(utils.AdminUrl+"/cluster/deploy", http.MethodPost, dryRunDisabled)

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not deploy clusters : %s. %s", res.Status, string(result))
		} else if !dryRunDisabled {
			fmt.Println("Clusters deployable.")
		} else {
			fmt.Println("Clusters deploying.")
		}
	}
}

func DeployFailedClusters() {
	utils.CheckAdminUrl()

	if utils.Validate("deployment") {
		res := deploy(utils.AdminUrl+"/cluster/deployFailedClusters", http.MethodPost, true)

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not deploy clusters : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("Clusters deploying.")
		}
	}
}

func deploy(url string, method string, dryRunDisabled bool) *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	body := bytes.NewBuffer([]byte(`{ "metadata": { "dry_run_deploy": true } }`))

	if dryRunDisabled {
		body = bytes.NewBuffer([]byte(`{ "metadata": { "dry_run_deploy": false } }`))
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

func ForceFailedDeploymentsToInternalErrorStatus() {
	utils.CheckAdminUrl()

	if utils.Validate("force deployment status") {
		res := deploy(utils.AdminUrl+"/deployment/forceFailedDeploymentsToInternalErrorStatus", http.MethodPost, true)
		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not force the deployments status : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("INTERNAL_ERROR status forced")
		}
	}
}
